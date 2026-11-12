package db

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Облигации (ОВДП) ручного счёта. В базе лежат только «тела» облигаций (ISIN, погашение, ставка, номинал)
// и покупки/продажи в журнале. Купоны, погашения, накопленный купон и стоимость позиций считаются на лету.

const bondsSchema = `
CREATE TABLE IF NOT EXISTS icu_bonds (
	account_id TEXT NOT NULL,
	isin TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	maturity TEXT NOT NULL,           -- YYYY-MM-DD
	coupon_rate REAL NOT NULL,        -- годовая ставка, % (16.44)
	nominal REAL NOT NULL DEFAULT 1000,
	PRIMARY KEY (account_id, isin)
);
`

const couponPeriodDays = 182 // купон раз в 182 дня, даты отсчитываются назад от погашения

const (
	KindBuy  = "buy"  // купили облигации: amount < 0 (с накопленным купоном), isin + qty
	KindSell = "sell" // продали: amount > 0, isin + qty
)

type Bond struct {
	AccountID  string  `json:"account_id"`
	IBAN       string  `json:"iban,omitempty"` // только во входящих запросах: счёт по IBAN
	ISIN       string  `json:"isin"`
	Name       string  `json:"name"`
	Maturity   string  `json:"maturity"`
	CouponRate float64 `json:"coupon_rate"`
	Nominal    float64 `json:"nominal"`
}

// Position — позиция по облигации на «сейчас».
type Position struct {
	Bond
	Qty        float64 `json:"qty"`
	Accrued    float64 `json:"accrued"`     // накопленный купон на одну бумагу
	Value      float64 `json:"value"`       // qty × (номинал + накопленный купон)
	NextDate   string  `json:"next_date"`   // ближайшая выплата
	NextAmount float64 `json:"next_amount"` // на всю позицию (купон; в дату погашения — вместе с номиналом)
}

// PayEvent — вычисляемая выплата: купон или погашение. Done — уже прошла (деньги в свободных средствах).
type PayEvent struct {
	Date   string  `json:"date"`
	Kind   string  `json:"kind"` // coupon | redemption
	ISIN   string  `json:"isin"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Qty    float64 `json:"qty"` // сколько бумаг получает выплату (на график — по маркеру на штуку)
	Done   bool    `json:"done"`
}

// AccountState — всё, что считается из журнала и облигаций на момент now.
type AccountState struct {
	Cash      float64 // свободные средства: журнал + полученные купоны и погашения
	Positions []Position
	Events    []PayEvent
	Balance   float64 // Cash + стоимость позиций
}

func (b Bond) maturityDate() (time.Time, error) {
	return time.Parse("2006-01-02", b.Maturity)
}

func utcDay(t time.Time) int64 {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400
}

func dayString(day int64) string { return time.Unix(day*86400, 0).UTC().Format("2006-01-02") }

// couponDays — все даты купонов (дни от эпохи), по возрастанию; последняя — погашение.
// Идём назад от погашения шагами по 182 дня, пока не уйдём раньше первой покупки (или не наберём 80 периодов).
func couponDays(mat time.Time, earliest int64) []int64 {
	m := utcDay(mat)
	var days []int64
	for k := 0; k < 80; k++ {
		d := m - int64(k*couponPeriodDays)
		days = append(days, d)
		if d < earliest {
			break
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })
	return days
}

func (b Bond) coupon() float64 { return b.Nominal * b.CouponRate / 100 / 2 }

// ComputeState считает состояние счёта: баланс, позиции, выплаты. Чистая функция (тестируется без БД).
func ComputeState(rows []LedgerRow, bonds []Bond, now time.Time) AccountState {
	var st AccountState
	for _, r := range rows {
		st.Cash += r.Amount
	}
	today := dayNum(now.Unix())

	// qtyBefore: сколько бумаг держали к началу дня day (покупки/продажи в этот день ещё не считаются)
	qtyBefore := func(isin string, day int64) float64 {
		var q float64
		for _, r := range rows {
			if r.ISIN != isin {
				continue
			}
			if dayNum(r.Time) >= day {
				continue
			}
			switch r.Kind {
			case KindBuy:
				q += r.Qty
			case KindSell:
				q -= r.Qty
			}
		}
		return q
	}
	earliest := int64(math.MaxInt64)
	for _, r := range rows {
		if d := dayNum(r.Time); d < earliest {
			earliest = d
		}
	}

	for _, b := range bonds {
		mat, err := b.maturityDate()
		if err != nil || b.Nominal <= 0 {
			continue
		}
		var held float64
		for _, r := range rows {
			if r.ISIN != b.ISIN {
				continue
			}
			switch r.Kind {
			case KindBuy:
				held += r.Qty
			case KindSell:
				held -= r.Qty
			}
		}
		days := couponDays(mat, earliest)
		matDay := utcDay(mat)
		matured := matDay <= today

		var nextDay int64
		for _, d := range days {
			isRedemption := d == matDay
			if d <= today {
				q := qtyBefore(b.ISIN, d)
				if q <= 0 {
					continue
				}
				amt := q * b.coupon()
				st.Cash += amt
				st.Events = append(st.Events, PayEvent{Date: dayString(d), Kind: "coupon", ISIN: b.ISIN, Name: b.Name, Amount: round2(amt), Qty: q, Done: true})
				if isRedemption {
					red := q * b.Nominal
					st.Cash += red
					st.Events = append(st.Events, PayEvent{Date: dayString(d), Kind: "redemption", ISIN: b.ISIN, Name: b.Name, Amount: round2(red), Qty: q, Done: true})
				}
				continue
			}
			if nextDay == 0 {
				nextDay = d
			}
			if held > 0 {
				st.Events = append(st.Events, PayEvent{Date: dayString(d), Kind: "coupon", ISIN: b.ISIN, Name: b.Name, Amount: round2(held * b.coupon()), Qty: held})
				if isRedemption {
					st.Events = append(st.Events, PayEvent{Date: dayString(d), Kind: "redemption", ISIN: b.ISIN, Name: b.Name, Amount: round2(held * b.Nominal), Qty: held})
				}
			}
		}

		if held <= 0 || matured {
			continue
		}
		p := Position{Bond: b, Qty: held}
		if nextDay != 0 {
			prev := nextDay - couponPeriodDays
			p.Accrued = round2(b.coupon() * float64(today-prev) / couponPeriodDays)
			p.NextDate = dayString(nextDay)
			p.NextAmount = round2(held * b.coupon())
			if nextDay == matDay {
				p.NextAmount = round2(held * (b.coupon() + b.Nominal))
			}
		}
		p.Value = round2(held * (b.Nominal + p.Accrued))
		st.Positions = append(st.Positions, p)
		st.Balance += p.Value
	}
	sort.SliceStable(st.Events, func(i, j int) bool { return st.Events[i].Date < st.Events[j].Date })
	st.Cash = round2(st.Cash)
	st.Balance = round2(st.Balance + st.Cash)
	return st
}

// ───────────────────────── хранение ─────────────────────────

func (s *Storage) initManual() error {
	if _, err := s.coreDB.Exec(manualSchema); err != nil {
		return err
	}
	if _, err := s.coreDB.Exec(bondsSchema); err != nil {
		return err
	}
	// журнал мог быть создан в фазе 1 без isin/qty
	for _, col := range []string{"isin TEXT NOT NULL DEFAULT ''", "qty REAL NOT NULL DEFAULT 0"} {
		if !hasColumn(s.coreDB, "icu_ledger", strings.Fields(col)[0]) {
			if _, err := s.coreDB.Exec(`ALTER TABLE icu_ledger ADD COLUMN ` + col); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Storage) ListBonds(accountID string) ([]Bond, error) {
	q := `SELECT account_id, isin, name, maturity, coupon_rate, nominal FROM icu_bonds`
	args := []any{}
	if accountID != "" {
		q += ` WHERE account_id = ?`
		args = append(args, accountID)
	}
	rows, err := s.coreDB.Query(q+` ORDER BY maturity, isin`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Bond{}
	for rows.Next() {
		var b Bond
		if err := rows.Scan(&b.AccountID, &b.ISIN, &b.Name, &b.Maturity, &b.CouponRate, &b.Nominal); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

func (s *Storage) SaveBond(b Bond) (Bond, error) {
	b.ISIN = strings.ToUpper(strings.TrimSpace(b.ISIN))
	b.Name = strings.TrimSpace(b.Name)
	if b.ISIN == "" {
		return b, errors.New("нужен ISIN")
	}
	mat, err := b.maturityDate()
	if err != nil {
		return b, errors.New("дата погашения: ГГГГ-ММ-ДД")
	}
	if b.Name == "" {
		b.Name = "ОВДП " + mat.Format("02.01.2006")
	}
	if b.CouponRate <= 0 {
		return b, errors.New("нужна ставка купона, %")
	}
	if b.Nominal == 0 {
		b.Nominal = 1000
	}
	if b.Nominal < 0 {
		return b, errors.New("номинал должен быть положительным")
	}
	if _, err := s.manualAccount(b.AccountID); err != nil {
		return b, err
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err = s.coreDB.Exec(`
		INSERT INTO icu_bonds(account_id, isin, name, maturity, coupon_rate, nominal) VALUES(?,?,?,?,?,?)
		ON CONFLICT(account_id, isin) DO UPDATE SET name=excluded.name, maturity=excluded.maturity,
			coupon_rate=excluded.coupon_rate, nominal=excluded.nominal`,
		b.AccountID, b.ISIN, b.Name, b.Maturity, b.CouponRate, b.Nominal)
	return b, err
}

// DeleteBond удаляет облигацию; нельзя, пока в журнале есть покупки/продажи по ней.
func (s *Storage) DeleteBond(accountID, isin string) error {
	var n int
	_ = s.coreDB.QueryRow(`SELECT COUNT(*) FROM icu_ledger WHERE account_id = ? AND isin = ?`, accountID, isin).Scan(&n)
	if n > 0 {
		return fmt.Errorf("по %s есть %d записей в журнале — сначала удалите их", isin, n)
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`DELETE FROM icu_bonds WHERE account_id = ? AND isin = ?`, accountID, isin)
	return err
}

// accountState — состояние ручного счёта на сейчас.
func (s *Storage) accountState(accountID string) (AccountState, error) {
	rows, err := s.loadLedger(accountID)
	if err != nil {
		return AccountState{}, err
	}
	bonds, err := s.ListBonds(accountID)
	if err != nil {
		return AccountState{}, err
	}
	return ComputeState(rows, bonds, time.Now()), nil
}

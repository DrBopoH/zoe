package db

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Брокерский счёт ICU. Его данные — только брокерский отчёт (pkg/icu): счёт и журнал лежат в cache.db и
// восстанавливаются кнопкой «Загрузить». core.db знает про счёт ровно то же, что про любой другой:
// алиас (имя файла) → IBAN. Ручных счетов, ручного журнала и сверки «призраков» больше нет.
//
// Баланс нигде не хранится — считается из журнала и справочника облигаций на чтении (ComputeState).

const (
	ICUAccountType = "icu"
	icuMFO         = "380946" // код банка ICU в IBAN (UA67 380946 …)
)

const icuCacheSchema = `
CREATE TABLE IF NOT EXISTS icu_flows (
	id TEXT PRIMARY KEY,                 -- стабильный ключ записи отчёта: повторная загрузка не плодит дублей
	account_id TEXT NOT NULL,
	time INTEGER NOT NULL,
	kind TEXT NOT NULL,
	amount REAL NOT NULL,                -- изменение свободных средств, со знаком
	note TEXT NOT NULL DEFAULT '',
	isin TEXT NOT NULL DEFAULT '',
	qty REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_icu_flows_acc ON icu_flows(account_id, time);
`

// Виды записей журнала (те же строки, что у pkg/icu).
const (
	KindDeposit  = "deposit"  // деньги пришли на счёт (+)
	KindWithdraw = "withdraw" // деньги ушли со счёта (−)
)

type LedgerRow struct {
	ID        string  `json:"id"`
	AccountID string  `json:"account_id"`
	Time      int64   `json:"time"` // unix
	Kind      string  `json:"kind"`
	Amount    float64 `json:"amount"` // изменение свободных средств, со знаком
	Note      string  `json:"note"`
	ISIN      string  `json:"isin,omitempty"` // buy/sell: облигация
	Qty       float64 `json:"qty,omitempty"`  // buy/sell: штук
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ICUAccountID — id счёта по номеру договора из отчёта («БО-261021-0058» → icu_2610210058).
// В отчёте нет ни IBAN, ни карты: договор — единственный стабильный ключ.
func ICUAccountID(contract string) string {
	var b strings.Builder
	for _, r := range contract {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return "icu_" + b.String()
}

func isICUIBAN(iban string) bool {
	n := normID(iban)
	return len(n) >= 10 && strings.HasPrefix(n, "UA") && n[4:10] == icuMFO
}

// ───────────────────────── журнал в кеше ─────────────────────────

// SaveICUFlows пишет записи отчёта (идемпотентно по id) и дату открытия счёта (дата договора).
func (s *Storage) SaveICUFlows(accountID string, openedAt int64, rows []LedgerRow) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, r := range rows {
		if _, err := tx.Exec(`
			INSERT INTO icu_flows(id, account_id, time, kind, amount, note, isin, qty) VALUES(?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET account_id=excluded.account_id, time=excluded.time, kind=excluded.kind,
				amount=excluded.amount, note=excluded.note, isin=excluded.isin, qty=excluded.qty`,
			r.ID, accountID, r.Time, r.Kind, r.Amount, r.Note, r.ISIN, r.Qty); err != nil {
			return err
		}
	}
	if openedAt > 0 {
		if _, err := tx.Exec(`UPDATE accounts SET opened_at=? WHERE id=? AND (COALESCE(opened_at,0)=0 OR opened_at>?)`,
			openedAt, accountID, openedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) loadLedger(accID string) ([]LedgerRow, error) {
	q := `SELECT id, account_id, time, kind, amount, note, isin, qty FROM icu_flows`
	args := []any{}
	if accID != "" {
		q += ` WHERE account_id = ?`
		args = append(args, accID)
	}
	rows, err := s.cacheDB.Query(q+` ORDER BY time, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []LedgerRow{}
	for rows.Next() {
		var r LedgerRow
		if err := rows.Scan(&r.ID, &r.AccountID, &r.Time, &r.Kind, &r.Amount, &r.Note, &r.ISIN, &r.Qty); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

// ───────────────────────── счета ─────────────────────────

// icuIBAN — IBAN брокерского счёта. В отчёте его нет, его знает только пользователь: IBAN, привязанный к алиасу
// с именем счёта (имя файла отчёта). Путь тот же, что у любого счёта: core.db → alias_accounts.
func (s *Storage) icuIBAN(a Account) string {
	rows, err := s.coreDB.Query(`SELECT account_identifier FROM alias_accounts WHERE alias_key = ? ORDER BY account_identifier`,
		aliasKey(cleanAccountName(a.Name)))
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && isIBANLike(id) && isICUIBAN(id) {
			return normID(id)
		}
	}
	return ""
}

// GetAllAccounts — счета кеша; у брокерского баланс и IBAN считаются на чтении.
// Внутри бэкенда (синк, индекс кеша) по-прежнему используется GetAccounts.
func (s *Storage) GetAllAccounts() ([]Account, error) {
	list, err := s.GetAccounts()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Type != ICUAccountType {
			continue
		}
		list[i].IBAN = s.icuIBAN(list[i])
		if st, err := s.accountState(list[i].ID); err == nil {
			list[i].Balance = st.Balance
		}
	}
	return list, nil
}

// ───────────────────────── распознавание переводов ─────────────────────────

// icuMatcher отвечает «на какой брокерский счёт ушла эта операция кеша». Только явные связи пользователя:
//  1. counter_iban операции равен IBAN счёта (IBAN привязан к алиасу счёта);
//  2. алиас контрагента привязан (alias → IBAN) к этому IBAN в обычном редакторе алиасов.
//
// Подстрок и «похожих» имён нет.
type icuMatcher struct {
	ibans   map[string]string // normID(IBAN счёта) → id счёта
	byAlias map[string]string // alias_key → id счёта
}

func (s *Storage) newIcuMatcher(accs []Account) *icuMatcher {
	m := &icuMatcher{ibans: map[string]string{}, byAlias: map[string]string{}}
	for _, a := range accs {
		if a.Type == ICUAccountType && a.IBAN != "" {
			m.ibans[normID(a.IBAN)] = a.ID
		}
	}
	if len(m.ibans) == 0 {
		return m
	}
	rows, err := s.coreDB.Query(`SELECT alias_key, account_identifier FROM alias_accounts`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var ak, id string
		if rows.Scan(&ak, &id) != nil || !isIBANLike(id) {
			continue
		}
		if acc, ok := m.ibans[normID(id)]; ok {
			m.byAlias[ak] = acc
		}
	}
	return m
}

func (m *icuMatcher) match(counterIBAN, counterName, description string) string {
	if counterIBAN != "" {
		if acc, ok := m.ibans[normID(counterIBAN)]; ok {
			return acc
		}
	}
	if key := txAliasKey(counterIBAN, counterName, description); key != "" {
		return m.byAlias[key]
	}
	return ""
}

// ───────────────────────── операции ─────────────────────────

func ledgerLabel(r LedgerRow) string {
	if r.Note != "" {
		return r.Note
	}
	switch r.Kind {
	case KindDeposit:
		return "Пополнение"
	case KindWithdraw:
		return "Вывод"
	case KindBuy:
		return "Покупка " + r.ISIN
	case KindSell:
		return "Продажа " + r.ISIN
	}
	return r.Kind
}

func isInternalKind(k string) bool { return k == KindBuy || k == KindSell }

// GetAllTransactions — операции кеша (с пометкой переводов на брокерский счёт) + записи журнала как операции счёта.
// Обе стороны перевода помечены Internal: это перемещение между своими счетами, а не доход/расход.
func (s *Storage) GetAllTransactions(limit int) ([]Transaction, error) {
	list, err := s.GetTransactions(limit)
	if err != nil {
		return nil, err
	}
	for i := range list { // внешние движения Binance — перемещение капитала в оценку портфеля, а не доход/расход месяца
		if list[i].AccountID == BinanceAccountID {
			list[i].Internal = true
		}
	}
	accs, err := s.GetAllAccounts()
	if err != nil {
		return nil, err
	}
	mt := s.newIcuMatcher(accs)
	for i := range list {
		t := &list[i]
		if acc := mt.match(t.CounterIBAN, t.CounterName, t.Description); acc != "" {
			t.LinkedAccountID, t.Internal = acc, true
		}
	}
	ledger, err := s.loadLedger("")
	if err != nil {
		return nil, err
	}
	if len(ledger) == 0 {
		return list, nil
	}
	for _, r := range ledger {
		if isInternalKind(r.Kind) {
			continue // покупки/продажи облигаций — внутри счёта, в поток операций не идут
		}
		list = append(list, Transaction{
			ID: "ledger_" + r.ID, AccountID: r.AccountID,
			Time: time.Unix(r.Time, 0).Format(time.RFC3339), Description: ledgerLabel(r),
			Amount: r.Amount, CurrencyCode: 980, OperationAmount: r.Amount, Internal: true,
		})
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].Time > list[j].Time })
	return list, nil
}

// ───────────────────────── обзор счёта ─────────────────────────

type ICUOverview struct {
	Account   Account     `json:"account"`
	Balance   float64     `json:"balance"`
	Cash      float64     `json:"cash"`
	Rows      []LedgerRow `json:"rows"`
	Positions []Position  `json:"positions"`
	Events    []PayEvent  `json:"events"`
	Bonds     []Bond      `json:"bonds"`
}

func dayNum(ts int64) int64 {
	t := time.Unix(ts, 0).In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400
}

// kindRank — порядок записей одного дня: деньги пришли → купили → продали → ушли.
func kindRank(k string) int {
	switch k {
	case KindDeposit:
		return 0
	case KindBuy:
		return 2
	case KindSell:
		return 3
	}
	return 4
}

// ICUOverview — журнал счёта, позиции и выплаты по графику. Только чтение: править нечего, всё из отчёта.
func (s *Storage) ICUOverview(accountID string) (ICUOverview, error) {
	var ov ICUOverview
	accs, err := s.GetAllAccounts()
	if err != nil {
		return ov, err
	}
	found := false
	for _, a := range accs {
		if a.ID == accountID && a.Type == ICUAccountType {
			ov.Account, found = a, true
		}
	}
	if !found {
		return ov, fmt.Errorf("счёт %q не найден", accountID)
	}
	rows, err := s.loadLedger(accountID)
	if err != nil {
		return ov, err
	}
	// новые сверху; внутри дня — по логике дня: пополнение → покупка → продажа → вывод, сверху то, что позже
	sort.SliceStable(rows, func(i, j int) bool {
		if di, dj := dayNum(rows[i].Time), dayNum(rows[j].Time); di != dj {
			return di > dj
		}
		if ri, rj := kindRank(rows[i].Kind), kindRank(rows[j].Kind); ri != rj {
			return ri > rj
		}
		if rows[i].Time != rows[j].Time {
			return rows[i].Time > rows[j].Time
		}
		return rows[i].ID > rows[j].ID
	})
	ov.Rows = rows
	st, err := s.accountState(accountID)
	if err != nil {
		return ov, err
	}
	ov.Balance, ov.Cash, ov.Positions, ov.Events = st.Balance, st.Cash, st.Positions, st.Events
	if ov.Positions == nil {
		ov.Positions = []Position{}
	}
	if ov.Events == nil {
		ov.Events = []PayEvent{}
	}
	if ov.Bonds, err = s.ListBonds(); err != nil {
		return ov, err
	}
	return ov, nil
}

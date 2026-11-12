package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ICU Trader и другие «ручные» счета: счёт и его журнал живут в core.db (кеш одноразовый, а это данные пользователя).
// Баланс нигде не хранится — считается из журнала на чтении.

const manualSchema = `
CREATE TABLE IF NOT EXISTS manual_accounts (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL DEFAULT 'invest',
	iban TEXT NOT NULL DEFAULT '',
	currency_code INTEGER NOT NULL DEFAULT 980,
	bank_key TEXT NOT NULL DEFAULT ''
);
-- Журнал счёта: amount — изменение свободных средств со знаком, src_tx_id — операция кеша, из которой запись принята.
CREATE TABLE IF NOT EXISTS icu_ledger (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id TEXT NOT NULL,
	time INTEGER NOT NULL,
	kind TEXT NOT NULL,
	amount REAL NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	src_tx_id TEXT NOT NULL DEFAULT '',
	isin TEXT NOT NULL DEFAULT '',
	qty REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_icu_ledger_acc ON icu_ledger(account_id, time);
`

// Виды записей журнала.
const (
	KindDeposit  = "deposit"  // деньги пришли на счёт (+)
	KindWithdraw = "withdraw" // деньги ушли со счёта (−)
	KindAdjust   = "adjust"   // ручная поправка (±)
)

// Статусы сверки записи журнала с кешем.
const (
	StatusGreen  = "green"  // пара в кеше найдена, суммы сходятся
	StatusOrange = "orange" // в кеше такой операции нет (не подтянулась, кеш стёрт или запись ручная)
	StatusRed    = "red"    // в кеше есть операция рядом, но суммы не сходятся
)

type ManualAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	IBAN     string `json:"iban"`
	Currency int    `json:"currency_code"`
	BankKey  string `json:"bank_key"`
}

type LedgerRow struct {
	ID        int64   `json:"id"`
	AccountID string  `json:"account_id"`
	Time      int64   `json:"time"` // unix
	Kind      string  `json:"kind"`
	Amount    float64 `json:"amount"` // изменение свободных средств, со знаком
	Note      string  `json:"note"`
	SrcTxID   string  `json:"src_tx_id,omitempty"`
	ISIN      string  `json:"isin,omitempty"` // buy/sell: облигация
	Qty       float64 `json:"qty,omitempty"`  // buy/sell: штук
}

func round2(v float64) float64  { return math.Round(v*100) / 100 }
func sameAmt(a, b float64) bool { return math.Abs(a-b) < 0.005 }

// ───────────────────────── ручные счета ─────────────────────────

func (s *Storage) ListManualAccounts() ([]ManualAccount, error) {
	s.ensureICUAccounts()
	rows, err := s.coreDB.Query(`SELECT id, name, type, iban, currency_code, bank_key FROM manual_accounts ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ManualAccount{}
	for rows.Next() {
		var m ManualAccount
		if err := rows.Scan(&m.ID, &m.Name, &m.Type, &m.IBAN, &m.Currency, &m.BankKey); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *Storage) manualAccount(id string) (ManualAccount, error) {
	var m ManualAccount
	err := s.coreDB.QueryRow(`SELECT id, name, type, iban, currency_code, bank_key FROM manual_accounts WHERE id = ?`, id).
		Scan(&m.ID, &m.Name, &m.Type, &m.IBAN, &m.Currency, &m.BankKey)
	if errors.Is(err, sql.ErrNoRows) {
		return m, fmt.Errorf("счёт %q не найден", id)
	}
	return m, err
}

// SaveManualAccount создаёт счёт (пустой ID) или обновляет существующий.
func (s *Storage) SaveManualAccount(m ManualAccount) (ManualAccount, error) {
	m.Name = strings.TrimSpace(m.Name)
	m.IBAN = strings.TrimSpace(m.IBAN)
	if m.Name == "" {
		return m, errors.New("нужно название счёта")
	}
	if m.IBAN != "" && !isIBANLike(m.IBAN) {
		return m, errors.New("некорректный IBAN")
	}
	if m.Type == "" {
		m.Type = "invest"
	}
	if m.Currency == 0 {
		m.Currency = 980
	}
	if m.BankKey == "" {
		m.BankKey = "icu"
	}
	if m.ID == "" {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return m, err
		}
		m.ID = "manual_" + hex.EncodeToString(b)
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`
		INSERT INTO manual_accounts(id, name, type, iban, currency_code, bank_key) VALUES(?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, type=excluded.type, iban=excluded.iban,
			currency_code=excluded.currency_code, bank_key=excluded.bank_key`,
		m.ID, m.Name, m.Type, m.IBAN, m.Currency, m.BankKey)
	return m, err
}

// DeleteManualAccount удаляет счёт вместе с его журналом.
func (s *Storage) DeleteManualAccount(id string) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM icu_ledger WHERE account_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM icu_bonds WHERE account_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM manual_accounts WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// manualToAccount — ручной счёт в виде обычного Account (баланс и «открыт» считаются из журнала).
func (s *Storage) manualToAccount(m ManualAccount) Account {
	var first int64
	_ = s.coreDB.QueryRow(`SELECT COALESCE(MIN(time),0) FROM icu_ledger WHERE account_id = ?`, m.ID).Scan(&first)
	st, _ := s.accountState(m.ID)
	return Account{
		ID: m.ID, Name: m.Name, Balance: st.Balance, Type: m.Type, Currency: m.Currency,
		IBAN: m.IBAN, Pans: []string{}, Source: "manual", OpenedAt: first,
		Manual: true, BankKey: m.BankKey,
	}
}

// GetAllAccounts — счета кеша + ручные. Внутри бэкенда (синк, индекс кеша) по-прежнему используется GetAccounts.
func (s *Storage) GetAllAccounts() ([]Account, error) {
	list, err := s.GetAccounts()
	if err != nil {
		return nil, err
	}
	man, err := s.ListManualAccounts()
	if err != nil {
		return nil, err
	}
	for _, m := range man {
		a := s.manualToAccount(m)
		a.HasDetail = true
		list = append(list, a)
	}
	if b, ok := s.binanceAccount(); ok {
		list = append(list, b)
	}
	return list, nil
}

// ───────────────────────── журнал ─────────────────────────

func normalizeLedger(r *LedgerRow) error {
	switch strings.TrimSpace(r.Kind) {
	case KindDeposit:
		r.Kind, r.Amount = KindDeposit, math.Abs(r.Amount)
	case KindWithdraw:
		r.Kind, r.Amount = KindWithdraw, -math.Abs(r.Amount)
	case KindAdjust:
		r.Kind = KindAdjust
	case KindBuy, KindSell:
		r.ISIN = strings.ToUpper(strings.TrimSpace(r.ISIN))
		if r.ISIN == "" || r.Qty <= 0 {
			return errors.New("для покупки/продажи нужны облигация и количество")
		}
		if r.Kind == KindBuy {
			r.Amount = -math.Abs(r.Amount)
		} else {
			r.Amount = math.Abs(r.Amount)
		}
	default:
		return fmt.Errorf("неизвестный вид записи %q", r.Kind)
	}
	r.Amount = round2(r.Amount)
	if r.Kind != KindBuy && r.Kind != KindSell {
		r.ISIN, r.Qty = "", 0
	}
	if r.Amount == 0 {
		return errors.New("сумма не может быть нулевой")
	}
	if r.Time <= 0 {
		return errors.New("нужна дата")
	}
	r.Note = strings.TrimSpace(r.Note)
	return nil
}

// SaveLedgerRow добавляет запись (ID == 0) или обновляет существующую. Знак суммы выставляется по виду записи.
func (s *Storage) SaveLedgerRow(r LedgerRow) (LedgerRow, error) {
	if err := normalizeLedger(&r); err != nil {
		return r, err
	}
	if _, err := s.manualAccount(r.AccountID); err != nil {
		return r, err
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	if r.ID == 0 {
		res, err := s.coreDB.Exec(`INSERT INTO icu_ledger(account_id, time, kind, amount, note, src_tx_id, isin, qty) VALUES(?,?,?,?,?,?,?,?)`,
			r.AccountID, r.Time, r.Kind, r.Amount, r.Note, r.SrcTxID, r.ISIN, r.Qty)
		if err != nil {
			return r, err
		}
		r.ID, _ = res.LastInsertId()
		return r, nil
	}
	res, err := s.coreDB.Exec(`UPDATE icu_ledger SET time=?, kind=?, amount=?, note=?, src_tx_id=?, isin=?, qty=? WHERE id=? AND account_id=?`,
		r.Time, r.Kind, r.Amount, r.Note, r.SrcTxID, r.ISIN, r.Qty, r.ID, r.AccountID)
	if err != nil {
		return r, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return r, errors.New("запись не найдена")
	}
	return r, nil
}

func (s *Storage) DeleteLedgerRow(id int64) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`DELETE FROM icu_ledger WHERE id = ?`, id)
	return err
}

func (s *Storage) loadLedger(accID string) ([]LedgerRow, error) {
	q := `SELECT id, account_id, time, kind, amount, note, src_tx_id, isin, qty FROM icu_ledger`
	args := []any{}
	if accID != "" {
		q += ` WHERE account_id = ?`
		args = append(args, accID)
	}
	rows, err := s.coreDB.Query(q+` ORDER BY time, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []LedgerRow{}
	for rows.Next() {
		var r LedgerRow
		if err := rows.Scan(&r.ID, &r.AccountID, &r.Time, &r.Kind, &r.Amount, &r.Note, &r.SrcTxID, &r.ISIN, &r.Qty); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

// ───────────────────────── распознавание переводов ─────────────────────────

// icuMatcher отвечает «в какой ручной счёт ушла эта операция кеша». Только явные связи пользователя:
//  1. counter_iban операции равен IBAN ручного счёта;
//  2. алиас контрагента привязан (alias → IBAN) к IBAN ручного счёта в обычном редакторе алиасов.
//
// Подстрок и «похожих» имён нет.
type icuMatcher struct {
	ibans   map[string]string // normID(IBAN ручного счёта) → id счёта
	byAlias map[string]string // alias_key → id счёта
}

func (s *Storage) newIcuMatcher(man []ManualAccount) *icuMatcher {
	m := &icuMatcher{ibans: map[string]string{}, byAlias: map[string]string{}}
	for _, a := range man {
		if a.IBAN != "" {
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
	return "Корректировка"
}

// GetAllTransactions — операции кеша (с пометкой переводов на ручные счета) + записи журналов как операции их счетов.
// Обе стороны перевода помечены Internal: это перемещение между своими счетами, а не доход/расход.
func (s *Storage) GetAllTransactions(limit int) ([]Transaction, error) {
	list, err := s.GetTransactions(limit)
	if err != nil {
		return nil, err
	}
	if bt, err := s.binanceTransactions(); err != nil {
		return nil, err
	} else if len(bt) > 0 {
		list = append(list, bt...)
	}
	man, err := s.ListManualAccounts()
	if err != nil {
		return nil, err
	}
	if len(man) == 0 {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Time > list[j].Time })
		return list, nil
	}
	mt := s.newIcuMatcher(man)
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
	for _, r := range ledger {
		if isInternalKind(r.Kind) {
			continue // покупки/продажи облигаций — внутри счёта, в поток операций не идут
		}
		list = append(list, Transaction{
			ID: fmt.Sprintf("ledger_%d", r.ID), AccountID: r.AccountID,
			Time: time.Unix(r.Time, 0).Format(time.RFC3339), Description: ledgerLabel(r),
			Amount: r.Amount, CurrencyCode: 980, OperationAmount: r.Amount, Internal: true,
		})
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].Time > list[j].Time })
	return list, nil
}

// ───────────────────────── сверка журнала с кешем ─────────────────────────

// Ghost — операция кеша, ушедшая на счёт (распознана по связи алиаса), которой нет в журнале.
type Ghost struct {
	TxID          string  `json:"tx_id"`
	Time          int64   `json:"time"`
	Amount        float64 `json:"amount"` // со стороны счёта ICU: перевод −100 на карте = +100 на ICU
	CounterName   string  `json:"counter_name"`
	Description   string  `json:"description"`
	FromAccountID string  `json:"from_account_id"`
}

type LedgerView struct {
	LedgerRow
	Status   string `json:"status,omitempty"`     // green | orange | red; у adjust пусто
	PairTxID string `json:"pair_tx_id,omitempty"` // операция кеша, с которой сверена запись
	Mismatch *Ghost `json:"mismatch,omitempty"`   // для red: что лежит в кеше рядом
}

type ICUOverview struct {
	Account   Account      `json:"account"`
	Balance   float64      `json:"balance"`
	Rows      []LedgerView `json:"rows"`
	Ghosts    []Ghost      `json:"ghosts"`
	Cash      float64      `json:"cash"`
	Positions []Position   `json:"positions"`
	Events    []PayEvent   `json:"events"`
	Bonds     []Bond       `json:"bonds"`
}

func isInternalKind(k string) bool { return k == KindBuy || k == KindSell }

// noRecon — записи, которые не сверяются с кешем (нет перевода на карте).
func noRecon(k string) bool { return k == KindAdjust || isInternalKind(k) }

func dayNum(ts int64) int64 {
	t := time.Unix(ts, 0).In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400
}

func dayDiff(a, b int64) int64 {
	d := dayNum(a) - dayNum(b)
	if d < 0 {
		d = -d
	}
	return d
}

func absInt(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func (s *Storage) icuCandidates(m ManualAccount, mt *icuMatcher) ([]Ghost, error) {
	rows, err := s.cacheDB.Query(`
		SELECT id, account_id, time, COALESCE(counter_iban,''), COALESCE(counter_name,''), COALESCE(description,''),
		       COALESCE(amount,0), COALESCE(currency_code,0)
		FROM transactions ORDER BY time, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ghost
	for rows.Next() {
		var g Ghost
		var ciban string
		var amount float64
		var cur int
		if err := rows.Scan(&g.TxID, &g.FromAccountID, &g.Time, &ciban, &g.CounterName, &g.Description, &amount, &cur); err != nil {
			return nil, err
		}
		if (cur != 980 && cur != 0) || amount == 0 || mt.match(ciban, g.CounterName, g.Description) != m.ID {
			continue
		}
		g.Amount = round2(-amount)
		out = append(out, g)
	}
	return out, rows.Err()
}

// ICUOverview — журнал счёта с цветом сверки и «призраками» (операции кеша без записи в журнале).
func (s *Storage) ICUOverview(accountID string) (ICUOverview, error) {
	var ov ICUOverview
	m, err := s.manualAccount(accountID)
	if err != nil {
		return ov, err
	}
	rows, err := s.loadLedger(accountID)
	if err != nil {
		return ov, err
	}
	cands, err := s.icuCandidates(m, s.newIcuMatcher([]ManualAccount{m}))
	if err != nil {
		return ov, err
	}

	views := make([]LedgerView, len(rows))
	for i, r := range rows {
		views[i] = LedgerView{LedgerRow: r}
	}
	used := make([]bool, len(cands))
	pair := func(v *LedgerView, j int, status string) {
		used[j], v.PairTxID, v.Status = true, cands[j].TxID, status
		if status == StatusRed {
			g := cands[j]
			v.Mismatch = &g
		}
	}

	// 1) запись принята из конкретной операции кеша — сверяем с ней (изменённая сумма даст red)
	for i := range views {
		v := &views[i]
		if noRecon(v.Kind) || v.SrcTxID == "" {
			continue
		}
		for j, c := range cands {
			if used[j] || c.TxID != v.SrcTxID {
				continue
			}
			if sameAmt(c.Amount, v.Amount) {
				pair(v, j, StatusGreen)
			} else {
				pair(v, j, StatusRed)
			}
			break
		}
	}
	// 2) та же сумма до копейки, дата в пределах ±1 дня
	nearest := func(v *LedgerView, ok func(Ghost) bool) int {
		best := -1
		for j, c := range cands {
			if used[j] || dayDiff(c.Time, v.Time) > 1 || !ok(c) {
				continue
			}
			if best < 0 || absInt(c.Time-v.Time) < absInt(cands[best].Time-v.Time) {
				best = j
			}
		}
		return best
	}
	for i := range views {
		v := &views[i]
		if noRecon(v.Kind) || v.Status != "" {
			continue
		}
		if j := nearest(v, func(c Ghost) bool { return sameAmt(c.Amount, v.Amount) }); j >= 0 {
			pair(v, j, StatusGreen)
		}
	}
	// 3) рядом по дате есть операция того же направления, но сумма другая — не сходится
	for i := range views {
		v := &views[i]
		if noRecon(v.Kind) || v.Status != "" {
			continue
		}
		if j := nearest(v, func(c Ghost) bool { return c.Amount*v.Amount > 0 }); j >= 0 {
			pair(v, j, StatusRed)
		}
	}
	// 4) остальные — в кеше не нашлось
	for i := range views {
		if v := &views[i]; !noRecon(v.Kind) && v.Status == "" {
			v.Status = StatusOrange
		}
	}

	ov.Ghosts = []Ghost{}
	for j, c := range cands {
		if !used[j] {
			ov.Ghosts = append(ov.Ghosts, c)
		}
	}
	// новые сверху; внутри одного дня (записи вводятся без часов, принятые из кеша — с часами) — по логике дня:
	// пополнение → покупка → продажа → вывод, а сверху оказывается то, что произошло позже
	sort.SliceStable(views, func(i, j int) bool {
		if di, dj := dayNum(views[i].Time), dayNum(views[j].Time); di != dj {
			return di > dj
		}
		if ri, rj := kindRank(views[i].Kind), kindRank(views[j].Kind); ri != rj {
			return ri > rj
		}
		if views[i].Time != views[j].Time {
			return views[i].Time > views[j].Time
		}
		return views[i].ID > views[j].ID
	})
	sort.SliceStable(ov.Ghosts, func(i, j int) bool { return ov.Ghosts[i].Time > ov.Ghosts[j].Time })
	ov.Rows = views
	ov.Account = s.manualToAccount(m)
	st, err := s.accountState(m.ID)
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
	if ov.Bonds, err = s.ListBonds(m.ID); err != nil {
		return ov, err
	}
	return ov, nil
}

// AcceptGhosts переносит операции кеша в журнал (кнопки «принять» / «принять все»). Возвращает, сколько принято.
func (s *Storage) AcceptGhosts(accountID string, txIDs []string, all bool) (int, error) {
	ov, err := s.ICUOverview(accountID)
	if err != nil {
		return 0, err
	}
	want := map[string]bool{}
	for _, id := range txIDs {
		want[id] = true
	}
	n := 0
	for _, g := range ov.Ghosts {
		if !all && !want[g.TxID] {
			continue
		}
		kind := KindDeposit
		if g.Amount < 0 {
			kind = KindWithdraw
		}
		if _, err := s.SaveLedgerRow(LedgerRow{AccountID: accountID, Time: g.Time, Kind: kind, Amount: g.Amount, SrcTxID: g.TxID}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// ───────────────────────── автосоздание счёта ICU ─────────────────────────

// icuMFO — код банка ICU в IBAN (UA67 380946 …). Счёт с таким IBAN, привязанный к алиасу, и есть счёт ICU.
const icuMFO = "380946"

func isICUIBAN(iban string) bool {
	n := normID(iban)
	return len(n) >= 10 && strings.HasPrefix(n, "UA") && n[4:10] == icuMFO
}

func (s *Storage) insertICUAccount(iban string) error {
	n := normID(iban)
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`INSERT OR IGNORE INTO manual_accounts(id, name, type, iban, currency_code, bank_key) VALUES(?,?,?,?,?,?)`,
		"icu_"+strings.ToLower(n), "ICU Trader", "invest", n, 980, "icu")
	return err
}

// ensureICUAccounts заводит ручной счёт для каждого IBAN банка ICU, привязанного к какому-либо алиасу.
// Руками счёт не создаётся: пользователь просто добавляет IBAN к алиасу.
func (s *Storage) ensureICUAccounts() {
	rows, err := s.coreDB.Query(`SELECT DISTINCT account_identifier FROM alias_accounts`)
	if err != nil {
		return
	}
	var ibans []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && isIBANLike(id) && isICUIBAN(id) {
			ibans = append(ibans, id)
		}
	}
	rows.Close()
	for _, iban := range ibans {
		var n int
		_ = s.coreDB.QueryRow(`SELECT COUNT(*) FROM manual_accounts WHERE upper(iban) = ?`, normID(iban)).Scan(&n)
		if n == 0 {
			_ = s.insertICUAccount(iban)
		}
	}
}

// ICUAccountByIBAN — ручной счёт по IBAN; для IBAN банка ICU создаётся на лету (облигации можно вводить
// сразу, ещё до сохранения алиаса).
func (s *Storage) ICUAccountByIBAN(iban string) (ManualAccount, error) {
	n := normID(iban)
	if n == "" {
		return ManualAccount{}, errors.New("нужен IBAN")
	}
	find := func() (ManualAccount, bool) {
		var m ManualAccount
		err := s.coreDB.QueryRow(`SELECT id, name, type, iban, currency_code, bank_key FROM manual_accounts WHERE upper(iban) = ?`, n).
			Scan(&m.ID, &m.Name, &m.Type, &m.IBAN, &m.Currency, &m.BankKey)
		return m, err == nil
	}
	if m, ok := find(); ok {
		return m, nil
	}
	if !isICUIBAN(n) {
		return ManualAccount{}, errors.New("это не счёт ICU")
	}
	if err := s.insertICUAccount(n); err != nil {
		return ManualAccount{}, err
	}
	m, _ := find()
	return m, nil
}

// kindRank — порядок записей одного дня: деньги пришли → купили → продали → ушли.
func kindRank(k string) int {
	switch k {
	case KindDeposit:
		return 0
	case KindAdjust:
		return 1
	case KindBuy:
		return 2
	case KindSell:
		return 3
	}
	return 4
}

package db

import (
	"math"
	"testing"
	"time"

	"economic/pkg/mono"
)

const (
	icuIban  = "UA503220010000026009000000001"
	icuAlias = "ТОВ «ІНВЕСТИЦІЙНИЙ КАПІТАЛ УКРАЇНА»"
)

// три реальных перевода с Raif на ICU: у банка нет IBAN контрагента, только имя
func seedICU(t *testing.T, s *Storage) ManualAccount {
	t.Helper()
	acc, err := s.SaveManualAccount(ManualAccount{Name: "ICU Trader", IBAN: icuIban})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveImportedAccount(mono.AccountDTO{ID: "raif_" + ibRaif, IBAN: ibRaif, Type: "raif"}, "Петро Іваненко.csv"); err != nil {
		t.Fatal(err)
	}
	seedICUTransfers(t, s)
	return acc
}

func ts(y int, m time.Month, d, h int) int64 {
	return time.Date(y, m, d, h, 0, 0, 0, time.Local).Unix()
}

func seedICUTransfers(t *testing.T, s *Storage) {
	t.Helper()
	if err := s.SaveTransactions("raif_"+ibRaif, []mono.StatementItemDTO{
		{ID: "x1", Time: ts(2026, 10, 23, 12), Amount: -204858, CounterName: icuAlias, CurrencyCode: 980},
		{ID: "x2", Time: ts(2026, 10, 30, 12), Amount: -103244, CounterName: icuAlias, CurrencyCode: 980},
		{ID: "x3", Time: ts(2026, 11, 5, 12), Amount: -207112, CounterName: icuAlias, CurrencyCode: 980},
		{ID: "x4", Time: ts(2026, 11, 6, 12), Amount: -50000, CounterName: "Магазин", CurrencyCode: 980},
	}); err != nil {
		t.Fatal(err)
	}
}

func linkICU(t *testing.T, s *Storage) {
	t.Helper()
	if err := s.LinkAccount("", icuAlias, icuIban, ""); err != nil {
		t.Fatal(err)
	}
}

func txByID(list []Transaction, id string) (Transaction, bool) {
	for _, x := range list {
		if x.ID == id {
			return x, true
		}
	}
	return Transaction{}, false
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func TestICUAccountAndBalance(t *testing.T) {
	s, _, _ := newS(t)
	acc, err := s.SaveManualAccount(ManualAccount{Name: "ICU Trader", IBAN: icuIban})
	if err != nil {
		t.Fatal(err)
	}
	if acc.Type != "invest" || acc.BankKey != "icu" || acc.Currency != 980 {
		t.Fatalf("defaults: %+v", acc)
	}
	if _, err := s.SaveManualAccount(ManualAccount{Name: "X", IBAN: "не iban"}); err == nil {
		t.Fatal("bad IBAN accepted")
	}
	// знак задаёт вид записи: вывод вводится положительным числом
	mustLedger := func(kind string, amount float64, day int) LedgerRow {
		r, err := s.SaveLedgerRow(LedgerRow{AccountID: acc.ID, Time: ts(2026, 10, day, 12), Kind: kind, Amount: amount})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	mustLedger(KindDeposit, 100.50, 1)
	w := mustLedger(KindWithdraw, 30, 2)
	if w.Amount != -30 {
		t.Fatalf("withdraw must be negative: %v", w.Amount)
	}
	mustLedger(KindAdjust, -0.5, 3)
	if _, err := s.SaveLedgerRow(LedgerRow{AccountID: acc.ID, Time: 1, Kind: "buy", Amount: 1}); err == nil {
		t.Fatal("unknown kind accepted")
	}
	if _, err := s.SaveLedgerRow(LedgerRow{AccountID: acc.ID, Time: 1, Kind: KindDeposit, Amount: 0}); err == nil {
		t.Fatal("zero amount accepted")
	}

	all, err := s.GetAllAccounts()
	if err != nil {
		t.Fatal(err)
	}
	var got *Account
	for i := range all {
		if all[i].ID == acc.ID {
			got = &all[i]
		}
	}
	if got == nil || !got.Manual || got.BankKey != "icu" || !near(got.Balance, 70) || got.IBAN != icuIban {
		t.Fatalf("manual account in list: %+v", got)
	}
	// кеш можно стирать — ручной счёт и журнал остаются
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	all, _ = s.GetAllAccounts()
	if len(all) != 1 || !near(all[0].Balance, 70) {
		t.Fatalf("after wipe: %+v", all)
	}
	// удаление записи и счёта
	if err := s.DeleteLedgerRow(w.ID); err != nil {
		t.Fatal(err)
	}
	all, _ = s.GetAllAccounts()
	if !near(all[0].Balance, 100) {
		t.Fatalf("after delete row: %+v", all[0])
	}
	if err := s.DeleteManualAccount(acc.ID); err != nil {
		t.Fatal(err)
	}
	if all, _ = s.GetAllAccounts(); len(all) != 0 {
		t.Fatalf("account not deleted: %+v", all)
	}
	if rows, _ := s.loadLedger(""); len(rows) != 0 {
		t.Fatalf("ledger not deleted: %+v", rows)
	}
}

// Без связи алиас → IBAN ICU переводы остаются обычными расходами; со связью — внутренние.
func TestICURecognitionOnlyViaAliasLink(t *testing.T) {
	s, _, _ := newS(t)
	acc := seedICU(t, s)

	txs, err := s.GetAllTransactions(1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range txs {
		if x.Internal || x.LinkedAccountID != "" {
			t.Fatalf("no link yet, but %s is internal", x.ID)
		}
	}

	linkICU(t, s)
	txs, _ = s.GetAllTransactions(1000)
	for _, id := range []string{"x1", "x2", "x3"} {
		x, ok := txByID(txs, id)
		if !ok || !x.Internal || x.LinkedAccountID != acc.ID {
			t.Fatalf("%s must be recognized: %+v", id, x)
		}
	}
	if x, _ := txByID(txs, "x4"); x.Internal {
		t.Fatal("unrelated shop payment became internal")
	}

	// снять связь — снова обычные расходы
	if err := s.UnlinkAccount(icuAlias, icuIban); err != nil {
		t.Fatal(err)
	}
	txs, _ = s.GetAllTransactions(1000)
	if x, _ := txByID(txs, "x1"); x.Internal {
		t.Fatal("unlinked alias still recognized")
	}
}

// IBAN контрагента, равный IBAN счёта, тоже явная связь (без алиаса).
func TestICURecognitionByCounterIBAN(t *testing.T) {
	s, _, _ := newS(t)
	acc := seedICU(t, s)
	if err := s.SaveTransactions("raif_"+ibRaif, []mono.StatementItemDTO{
		{ID: "y1", Time: ts(2026, 11, 7, 12), Amount: -1000, CounterName: "Інше імʼя", CounterIban: icuIban, CurrencyCode: 980},
	}); err != nil {
		t.Fatal(err)
	}
	txs, _ := s.GetAllTransactions(1000)
	if x, _ := txByID(txs, "y1"); !x.Internal || x.LinkedAccountID != acc.ID {
		t.Fatalf("counter IBAN match: %+v", x)
	}
}

func TestICUGhostsAcceptAndCandle(t *testing.T) {
	s, _, _ := newS(t)
	acc := seedICU(t, s)
	linkICU(t, s)

	ov, err := s.ICUOverview(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Rows) != 0 || len(ov.Ghosts) != 3 {
		t.Fatalf("expected 3 ghosts, got rows=%d ghosts=%d", len(ov.Rows), len(ov.Ghosts))
	}
	// сторона ICU: −2071.12 на карте = +2071.12 на ICU, новые сверху
	if !near(ov.Ghosts[0].Amount, 2071.12) || ov.Ghosts[0].TxID != "x3" {
		t.Fatalf("ghost order/amount: %+v", ov.Ghosts)
	}

	// принять один, потом все
	if n, err := s.AcceptGhosts(acc.ID, []string{"x1"}, false); err != nil || n != 1 {
		t.Fatalf("accept one: %d %v", n, err)
	}
	ov, _ = s.ICUOverview(acc.ID)
	if len(ov.Rows) != 1 || ov.Rows[0].Status != StatusGreen || ov.Rows[0].PairTxID != "x1" || len(ov.Ghosts) != 2 {
		t.Fatalf("after one: %+v", ov)
	}
	if n, _ := s.AcceptGhosts(acc.ID, nil, true); n != 2 {
		t.Fatalf("accept all: %d", n)
	}
	ov, _ = s.ICUOverview(acc.ID)
	if len(ov.Ghosts) != 0 || len(ov.Rows) != 3 || !near(ov.Balance, 5152.14) {
		t.Fatalf("after all: ghosts=%d rows=%d balance=%v", len(ov.Ghosts), len(ov.Rows), ov.Balance)
	}
	for _, r := range ov.Rows {
		if r.Status != StatusGreen || r.Kind != KindDeposit {
			t.Fatalf("row must be green deposit: %+v", r)
		}
	}

	// свеча: в день перевода обе стороны суммарно дают 0, значит красной свечи нет
	txs, _ := s.GetAllTransactions(1000)
	day := time.Unix(ts(2026, 10, 23, 12), 0).Format("2006-01-02")
	var sum float64
	var internal int
	for _, x := range txs {
		if len(x.Time) >= 10 && x.Time[:10] == day {
			sum += x.Amount
			if x.Internal {
				internal++
			}
		}
	}
	if !near(sum, 0) || internal != 2 {
		t.Fatalf("day net must be 0 with both sides internal: sum=%v internal=%d", sum, internal)
	}
	// записи журнала — операции счёта ICU
	if x, ok := txByID(txs, "ledger_1"); !ok || x.AccountID != acc.ID || !x.Internal || !near(x.Amount, 2048.58) {
		t.Fatalf("ledger tx: %+v", x)
	}
}

func TestICUReconcileColors(t *testing.T) {
	s, _, _ := newS(t)
	acc := seedICU(t, s)
	linkICU(t, s)

	add := func(kind string, amount float64, t0 int64, src string) LedgerRow {
		r, err := s.SaveLedgerRow(LedgerRow{AccountID: acc.ID, Time: t0, Kind: kind, Amount: amount, SrcTxID: src})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	// зелёная: сумма до копейки, дата на день раньше
	green := add(KindDeposit, 2048.58, ts(2026, 10, 22, 20), "")
	// красная: опечатка в сумме (1032.44 → 1032.64), дата та же
	red := add(KindDeposit, 1032.64, ts(2026, 10, 30, 12), "")
	// оранжевая: в кеше такого нет
	orange := add(KindDeposit, 777, ts(2026, 9, 1, 12), "")
	// поправка без сверки
	adj := add(KindAdjust, -1, ts(2026, 11, 1, 12), "")

	ov, err := s.ICUOverview(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	st := map[int64]LedgerView{}
	for _, r := range ov.Rows {
		st[r.ID] = r
	}
	if g := st[green.ID]; g.Status != StatusGreen || g.PairTxID != "x1" {
		t.Fatalf("green: %+v", g)
	}
	if r := st[red.ID]; r.Status != StatusRed || r.Mismatch == nil || r.Mismatch.TxID != "x2" || !near(r.Mismatch.Amount, 1032.44) {
		t.Fatalf("red: %+v", r)
	}
	if o := st[orange.ID]; o.Status != StatusOrange {
		t.Fatalf("orange: %+v", o)
	}
	if a := st[adj.ID]; a.Status != "" {
		t.Fatalf("adjust must have no status: %+v", a)
	}
	// x1 и x2 заняты парами, x3 — призрак
	if len(ov.Ghosts) != 1 || ov.Ghosts[0].TxID != "x3" {
		t.Fatalf("ghosts: %+v", ov.Ghosts)
	}

	// исправили сумму — стало зелёным и призрака нет
	red.Amount = 1032.44
	if _, err := s.SaveLedgerRow(red); err != nil {
		t.Fatal(err)
	}
	ov, _ = s.ICUOverview(acc.ID)
	for _, r := range ov.Rows {
		if r.ID == red.ID && r.Status != StatusGreen {
			t.Fatalf("fixed row must be green: %+v", r)
		}
	}
}

// Запись, принятая из операции кеша, после очистки кеша оранжевая, а после повторной загрузки снова зелёная.
func TestICUWipeCacheKeepsLedger(t *testing.T) {
	s, _, _ := newS(t)
	acc := seedICU(t, s)
	linkICU(t, s)
	if n, err := s.AcceptGhosts(acc.ID, nil, true); err != nil || n != 3 {
		t.Fatalf("accept: %d %v", n, err)
	}
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	ov, _ := s.ICUOverview(acc.ID)
	if len(ov.Rows) != 3 || len(ov.Ghosts) != 0 || !near(ov.Balance, 5152.14) {
		t.Fatalf("after wipe: %+v", ov)
	}
	for _, r := range ov.Rows {
		if r.Status != StatusOrange {
			t.Fatalf("after wipe row must be orange: %+v", r)
		}
	}
	// повторная загрузка выписки
	if err := s.SaveImportedAccount(mono.AccountDTO{ID: "raif_" + ibRaif, IBAN: ibRaif, Type: "raif"}, "Петро Іваненко.csv"); err != nil {
		t.Fatal(err)
	}
	seedICUTransfers(t, s)
	ov, _ = s.ICUOverview(acc.ID)
	for _, r := range ov.Rows {
		if r.Status != StatusGreen {
			t.Fatalf("after reload row must be green: %+v", r)
		}
	}
	if len(ov.Ghosts) != 0 {
		t.Fatalf("no ghosts expected: %+v", ov.Ghosts)
	}
}

// Входящий перевод от алиаса ICU (вывод денег) — зеркало с минусом на стороне ICU.
func TestICUWithdrawGhost(t *testing.T) {
	s, _, _ := newS(t)
	acc := seedICU(t, s)
	linkICU(t, s)
	if err := s.SaveTransactions("raif_"+ibRaif, []mono.StatementItemDTO{
		{ID: "w1", Time: ts(2026, 11, 8, 12), Amount: 50000, CounterName: icuAlias, CurrencyCode: 980},
	}); err != nil {
		t.Fatal(err)
	}
	ov, _ := s.ICUOverview(acc.ID)
	var w *Ghost
	for i := range ov.Ghosts {
		if ov.Ghosts[i].TxID == "w1" {
			w = &ov.Ghosts[i]
		}
	}
	if w == nil || !near(w.Amount, -500) {
		t.Fatalf("withdraw ghost: %+v", ov.Ghosts)
	}
	if _, err := s.AcceptGhosts(acc.ID, []string{"w1"}, false); err != nil {
		t.Fatal(err)
	}
	ov, _ = s.ICUOverview(acc.ID)
	if ov.Rows[0].Kind != KindWithdraw || !near(ov.Rows[0].Amount, -500) || ov.Rows[0].Status != StatusGreen {
		t.Fatalf("accepted withdraw: %+v", ov.Rows[0])
	}
}

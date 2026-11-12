package db

import (
	"strings"
	"testing"
	"time"

	"economic/pkg/mono"
)

const (
	icuIban   = "UA673809460000000000000005004"       // IBAN брокерского счёта: в отчёте его нет, его знает только пользователь
	icuAlias  = "ТОВ «ІНВЕСТИЦІЙНИЙ КАПІТАЛ УКРАЇНА»" // контрагент в выписке Райфа
	icuFile   = "ОВГЗ Тест.pdf"                       // имя файла отчёта = алиас счёта
	icuAccID  = "icu_2610210000"
	icuOpened = "БО-261021-0000"
)

func ts(y int, m time.Month, d, h int) int64 {
	return time.Date(y, m, d, h, 0, 0, 0, time.Local).Unix()
}

// отчёт: три пополнения и три покупки (как в реальном образце), открытие счёта 21.10.2026
func reportRows() []LedgerRow {
	return []LedgerRow{
		{ID: "op|23.10.2026|deposit|204858|1", Time: ts(2026, 10, 23, 12), Kind: KindDeposit, Amount: 2048.58, Note: "Зарахування ГК"},
		{ID: "ord:ЗМФ-261023-1269", Time: ts(2026, 10, 23, 12), Kind: KindBuy, Amount: -2048.58, ISIN: "UA4000239131", Qty: 2},
		{ID: "op|30.10.2026|deposit|103244|1", Time: ts(2026, 10, 30, 12), Kind: KindDeposit, Amount: 1032.44, Note: "Зарахування ГК"},
		{ID: "ord:ЗМФ-261030-1981", Time: ts(2026, 10, 30, 12), Kind: KindBuy, Amount: -1032.44, ISIN: "UA4000239081", Qty: 1},
		{ID: "op|05.11.2026|deposit|207112|1", Time: ts(2026, 11, 5, 12), Kind: KindDeposit, Amount: 2071.12, Note: "Зарахування ГК"},
		{ID: "ord:ЗМФ-261105-2320", Time: ts(2026, 11, 6, 12), Kind: KindBuy, Amount: -2071.12, ISIN: "UA4000239081", Qty: 2},
	}
}

// как это делает importICUReport: счёт из отчёта (имя = имя файла) + журнал
func seedReport(t *testing.T, s *Storage) {
	t.Helper()
	if err := s.SaveImportedAccount(mono.AccountDTO{ID: icuAccID, Type: ICUAccountType, CurrencyCode: 980}, icuFile); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveICUFlows(icuAccID, ts(2026, 10, 21, 12), reportRows()); err != nil {
		t.Fatal(err)
	}
}

// три реальных перевода с Raif на ICU: у банка нет IBAN контрагента, только имя
func seedRaifTransfers(t *testing.T, s *Storage) {
	t.Helper()
	if err := s.SaveImportedAccount(mono.AccountDTO{ID: "raif_" + ibRaif, IBAN: ibRaif, Type: "raif"}, "Петро Іваненко.csv"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTransactions("raif_"+ibRaif, []mono.StatementItemDTO{
		{ID: "x1", Time: ts(2026, 10, 23, 12), Amount: -204858, CounterName: icuAlias, CurrencyCode: 980},
		{ID: "x2", Time: ts(2026, 10, 30, 12), Amount: -103244, CounterName: icuAlias, CurrencyCode: 980},
		{ID: "x3", Time: ts(2026, 11, 5, 12), Amount: -207112, CounterName: icuAlias, CurrencyCode: 980},
		{ID: "x4", Time: ts(2026, 11, 6, 12), Amount: -50000, CounterName: "Магазин", CurrencyCode: 980},
	}); err != nil {
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

func icuAccount(t *testing.T, s *Storage) Account {
	t.Helper()
	accs, err := s.GetAllAccounts()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range accs {
		if a.Type == ICUAccountType {
			return a
		}
	}
	t.Fatalf("счёта ICU нет: %+v", accs)
	return Account{}
}

func TestICUAccountIDFromContract(t *testing.T) {
	if got := ICUAccountID("БО-261021-0058"); got != "icu_2610210058" {
		t.Fatal(got)
	}
}

// Счёт и журнал — кеш: сброс кеша стирает их, «Загрузить» восстанавливает; core.db не трогается.
func TestICUIsCacheAndWipeClearsIt(t *testing.T) {
	s, _, _ := newS(t)
	seedReport(t, s)
	if _, err := s.SaveBond(bond081); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkAccount("", icuFile, icuIban, ""); err != nil {
		t.Fatal(err)
	}
	a := icuAccount(t, s)
	if a.BankKey != "icu" || !a.HasDetail || a.OpenedAt != ts(2026, 10, 21, 12) {
		t.Fatalf("счёт: %+v", a)
	}
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if accs, _ := s.GetAllAccounts(); len(accs) != 0 {
		t.Fatalf("после сброса кеша счетов быть не должно: %+v", accs)
	}
	if rows, _ := s.loadLedger(""); len(rows) != 0 {
		t.Fatalf("журнал — кеш, должен стереться: %+v", rows)
	}
	if b, _ := s.ListBonds(); len(b) != 1 {
		t.Fatal("справочник облигаций — core, сброс кеша его не трогает")
	}
	// привязка алиаса в core пережила сброс: после «Загрузить» IBAN вернётся сам
	seedReport(t, s)
	if got := icuAccount(t, s).IBAN; got != icuIban {
		t.Fatalf("IBAN после повторной загрузки: %q", got)
	}
}

func TestICUFlowsAreIdempotent(t *testing.T) {
	s, _, _ := newS(t)
	seedReport(t, s)
	seedReport(t, s) // тот же (или перекрывающийся) отчёт повторно
	if rows, _ := s.loadLedger(icuAccID); len(rows) != 6 {
		t.Fatalf("записей %d, ждали 6", len(rows))
	}
}

// IBAN в отчёте нет: счёт получает его только из алиаса с именем файла — пути core.db, как у любого счёта.
func TestICUIBANComesOnlyFromFileAlias(t *testing.T) {
	s, _, _ := newS(t)
	seedReport(t, s)
	if got := icuAccount(t, s).IBAN; got != "" {
		t.Fatalf("до привязки IBAN быть не должно: %q", got)
	}
	// алиас контрагента из выписки Райфа — другое имя: счётом не становится
	if err := s.LinkAccount("", icuAlias, icuIban, ""); err != nil {
		t.Fatal(err)
	}
	if got := icuAccount(t, s).IBAN; got != "" {
		t.Fatalf("чужой алиас не даёт счёту IBAN: %q", got)
	}
	// IBAN не банка ICU к алиасу файла — тоже не счёт ICU
	if err := s.LinkAccount("", icuFile, "UA583003350000000026001234567", ""); err != nil {
		t.Fatal(err)
	}
	if got := icuAccount(t, s).IBAN; got != "" {
		t.Fatalf("IBAN чужого банка: %q", got)
	}
	// второй алиас на тот же IBAN: файл становится «вторым алиасом» счёта, первый (контрагент) остаётся
	if err := s.LinkAccount("", icuFile, icuIban, ""); err != nil {
		t.Fatal(err)
	}
	if got := icuAccount(t, s).IBAN; got != icuIban {
		t.Fatalf("IBAN: %q", got)
	}
}

func TestICUTransferRecognitionViaAliasLinks(t *testing.T) {
	s, _, _ := newS(t)
	seedReport(t, s)
	seedRaifTransfers(t, s)

	txs, _ := s.GetAllTransactions(1000)
	if x, _ := txByID(txs, "x1"); x.Internal {
		t.Fatal("без привязок перевод — обычный расход")
	}
	// привязали IBAN к алиасу файла и к алиасу контрагента
	if err := s.LinkAccount("", icuFile, icuIban, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkAccount("", icuAlias, icuIban, ""); err != nil {
		t.Fatal(err)
	}
	txs, _ = s.GetAllTransactions(1000)
	for _, id := range []string{"x1", "x2", "x3"} {
		if x, _ := txByID(txs, id); !x.Internal || x.LinkedAccountID != icuAccID {
			t.Fatalf("%s: перевод на брокерский счёт должен быть внутренним: %+v", id, x)
		}
	}
	if x, _ := txByID(txs, "x4"); x.Internal {
		t.Fatal("магазин — не перевод на счёт")
	}
	// записи журнала — операции счёта; покупки облигаций в поток не идут
	var deposits int
	for _, x := range txs {
		if x.AccountID != icuAccID {
			continue
		}
		if !x.Internal || strings.Contains(x.Description, "ЗМФ") {
			t.Fatalf("операция журнала: %+v", x)
		}
		deposits++
	}
	if deposits != 3 {
		t.Fatalf("пополнений в потоке %d, ждали 3", deposits)
	}
}

func TestICUBalanceFromReportAndBonds(t *testing.T) {
	s, _, _ := newS(t)
	seedReport(t, s)
	// справочника нет: позиции по себестоимости, ничего не теряется
	a := icuAccount(t, s)
	if a.Balance != 5152.14 {
		t.Fatalf("баланс без условий облигаций: %v", a.Balance)
	}
	ov, err := s.ICUOverview(icuAccID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Positions) != 2 || !ov.Positions[0].NoTerms || ov.Cash != 0 {
		t.Fatalf("overview: cash=%v pos=%+v", ov.Cash, ov.Positions)
	}
	// заданы условия — баланс считается как «номинал + накопленный купон»
	if _, err := s.SaveBond(Bond{ISIN: "UA4000239081", Maturity: "2030-02-20", CouponRate: 16.44}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveBond(Bond{ISIN: "UA4000239131", Maturity: "2029-03-21", CouponRate: 16}); err != nil {
		t.Fatal(err)
	}
	ov, _ = s.ICUOverview(icuAccID)
	for _, p := range ov.Positions {
		if p.NoTerms || p.Maturity == "" {
			t.Fatalf("условия заданы, а позиция без них: %+v", p)
		}
	}
	if a = icuAccount(t, s); a.Balance != ov.Balance {
		t.Fatalf("баланс счёта %v, overview %v", a.Balance, ov.Balance)
	}
	if _, err := s.ICUOverview("icu_nope"); err == nil {
		t.Fatal("чужой id — ошибка")
	}
}

// покупка в день пополнения стоит над пополнением (сверху то, что произошло позже в логике дня)
func TestICUJournalOrderWithinDay(t *testing.T) {
	s, _, _ := newS(t)
	seedReport(t, s)
	ov, err := s.ICUOverview(icuAccID)
	if err != nil || len(ov.Rows) != 6 {
		t.Fatalf("%v %+v", err, ov.Rows)
	}
	if ov.Rows[0].Kind != KindBuy || ov.Rows[0].Time != ts(2026, 11, 6, 12) {
		t.Fatalf("сверху последняя покупка: %+v", ov.Rows[0])
	}
	// 30.10: покупка выше пополнения
	var order []string
	for _, r := range ov.Rows {
		if dayNum(r.Time) == dayNum(ts(2026, 10, 30, 12)) {
			order = append(order, r.Kind)
		}
	}
	if strings.Join(order, ",") != "buy,deposit" {
		t.Fatalf("порядок внутри дня: %v", order)
	}
}

func TestBondReferenceValidationAndGuard(t *testing.T) {
	s, _, _ := newS(t)
	if bb, err := s.SaveBond(Bond{ISIN: "ua4000239131", Maturity: "2029-03-21", CouponRate: 16}); err != nil || bb.ISIN != "UA4000239131" || bb.Name != "ОВДП 21.03.2029" || bb.Nominal != 1000 {
		t.Fatalf("имя и номинал по умолчанию: %+v %v", bb, err)
	}
	if _, err := s.SaveBond(Bond{ISIN: "X", Maturity: "bad", CouponRate: 1}); err == nil {
		t.Fatal("кривая дата погашения должна отклоняться")
	}
	if _, err := s.SaveBond(Bond{ISIN: "X", Maturity: "2029-03-21"}); err == nil {
		t.Fatal("ставка обязательна")
	}
	seedReport(t, s)
	if err := s.DeleteBond("UA4000239131"); err == nil {
		t.Fatal("облигацию с записями в журнале удалять нельзя")
	}
	if _, err := s.SaveBond(Bond{ISIN: "UA0000000001", Maturity: "2029-03-21", CouponRate: 16}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBond("UA0000000001"); err != nil {
		t.Fatal(err)
	}
}

// Старый ручной слой не удаляется (core.db — данные пользователя): условия облигаций переезжают в справочник,
// таблицы переименовываются в *_legacy, повторный запуск безопасен.
func TestLegacyICUTablesAreKeptAndBondsCopied(t *testing.T) {
	s, _, _ := newS(t)
	for _, q := range []string{
		`CREATE TABLE manual_accounts (id TEXT PRIMARY KEY, name TEXT)`,
		`INSERT INTO manual_accounts VALUES ('icu_x','ICU Trader')`,
		`CREATE TABLE icu_ledger (id INTEGER PRIMARY KEY, account_id TEXT, amount REAL)`,
		`INSERT INTO icu_ledger VALUES (1,'icu_x',100)`,
		`CREATE TABLE icu_bonds (account_id TEXT NOT NULL, isin TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', maturity TEXT NOT NULL, coupon_rate REAL NOT NULL, nominal REAL NOT NULL DEFAULT 1000, PRIMARY KEY (account_id, isin))`,
		`INSERT INTO icu_bonds VALUES ('icu_x','UA4000239081','ОВДП 20.02.2030','2030-02-20',16.44,1000)`,
	} {
		if _, err := s.coreDB.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ { // повторный запуск безопасен
		if err := s.initICU(); err != nil {
			t.Fatal(err)
		}
	}
	bonds, _ := s.ListBonds()
	if len(bonds) != 1 || bonds[0].ISIN != "UA4000239081" || bonds[0].CouponRate != 16.44 {
		t.Fatalf("справочник: %+v", bonds)
	}
	for _, tb := range legacyICUCore {
		if tableExists(s.coreDB, tb) || !tableExists(s.coreDB, tb+"_legacy") {
			t.Fatalf("%s должна быть переименована в %s_legacy", tb, tb)
		}
	}
	var amount float64
	if err := s.coreDB.QueryRow(`SELECT amount FROM icu_ledger_legacy WHERE id=1`).Scan(&amount); err != nil || amount != 100 {
		t.Fatalf("данные старого журнала должны сохраниться: %v %v", amount, err)
	}
}

// Карточка брокерского счёта: свободные деньги отдельно от стоимости облигаций, вместе они дают баланс счёта.
func TestICUPortfolioCashAndBonds(t *testing.T) {
	s, _, _ := newS(t)
	seedReport(t, s)
	acc := icuAccount(t, s)
	p := acc.Portfolio
	if p == nil || p.Kind != "bonds" {
		t.Fatalf("состав счёта: %+v", p)
	}
	if d := p.Cash + p.Invested - acc.Balance; d > 0.011 || d < -0.011 {
		t.Fatalf("деньги %.2f + облигации %.2f ≠ баланс %.2f", p.Cash, p.Invested, acc.Balance)
	}
}

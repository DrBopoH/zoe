package db

import (
	"strings"
	"testing"
	"time"
)

var bond081 = Bond{ISIN: "UA4000239081", Name: "ОВДП 20.02.2030", Maturity: "2030-02-20", CouponRate: 16.44, Nominal: 1000}

func lrow(y int, m time.Month, d int, kind string, amount float64, isin string, qty float64) LedgerRow {
	return LedgerRow{Time: ts(y, m, d, 12), Kind: kind, Amount: amount, ISIN: isin, Qty: qty}
}

func TestBondScheduleDatesAndAccrued(t *testing.T) {
	mat, _ := bond081.maturityDate()
	days := couponDays(mat, utcDay(time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)))
	var got []string
	for _, d := range days {
		if d > utcDay(time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)) {
			got = append(got, dayString(d))
		}
	}
	want := "2027-02-24 2027-08-25 2028-02-23 2028-08-23 2029-02-21 2029-08-22 2030-02-20"
	if strings.Join(got, " ") != want {
		t.Fatalf("даты купонов: %v", got)
	}
	rows := []LedgerRow{
		lrow(2026, 11, 5, KindDeposit, 3200, "", 0),
		lrow(2026, 11, 5, KindBuy, -3070, bond081.ISIN, 3),
	}
	st := ComputeState(rows, []Bond{bond081}, time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local))
	if len(st.Positions) != 1 || st.Positions[0].Qty != 3 {
		t.Fatalf("позиции: %+v", st.Positions)
	}
	p := st.Positions[0]
	if p.Accrued != 34.33 || p.Value != 3102.99 {
		t.Fatalf("накопленный/стоимость: %v %v", p.Accrued, p.Value)
	}
	if st.Cash != 130 || st.Balance != 3232.99 {
		t.Fatalf("свободные %v баланс %v", st.Cash, st.Balance)
	}
	for _, e := range st.Events {
		if e.Qty != 3 {
			t.Fatalf("qty выплаты: %+v", e)
		}
	}
	if p.NextDate != "2027-02-24" || p.NextAmount != 246.6 {
		t.Fatalf("следующая выплата: %v %v", p.NextDate, p.NextAmount)
	}
	if n := len(st.Events); n != 7*1+1 { // 6 купонов + последний купон + погашение
		t.Fatalf("ожидалось 8 выплат, есть %d: %+v", n, st.Events)
	}
}

func TestBondCouponAndRedemptionBecomeCash(t *testing.T) {
	rows := []LedgerRow{
		lrow(2026, 11, 5, KindDeposit, 3200, "", 0),
		lrow(2026, 11, 5, KindBuy, -3070, bond081.ISIN, 3),
	}
	st := ComputeState(rows, []Bond{bond081}, time.Date(2027, 2, 25, 12, 0, 0, 0, time.Local))
	if st.Cash != 130+246.6 {
		t.Fatalf("после купона свободные = %v", st.Cash)
	}
	done := 0
	for _, e := range st.Events {
		if e.Done {
			done++
		}
	}
	if done != 1 {
		t.Fatalf("выплат прошло: %d", done)
	}
	// после погашения позиции нет, всё в деньгах: 130 + 7 купонов×246.6 + 3000
	st = ComputeState(rows, []Bond{bond081}, time.Date(2030, 2, 21, 12, 0, 0, 0, time.Local))
	if len(st.Positions) != 0 {
		t.Fatalf("позиция после погашения: %+v", st.Positions)
	}
	if want := round2(130 + 7*246.6 + 3000); st.Cash != want || st.Balance != want {
		t.Fatalf("после погашения %v / %v, ждали %v", st.Cash, st.Balance, want)
	}
}

func TestBondBoughtOnCouponDayGetsNoCoupon(t *testing.T) {
	rows := []LedgerRow{lrow(2027, 2, 24, KindBuy, -1000, bond081.ISIN, 1)}
	st := ComputeState(rows, []Bond{bond081}, time.Date(2027, 2, 25, 12, 0, 0, 0, time.Local))
	if st.Cash != -1000 {
		t.Fatalf("купон за день покупки не положен: %v", st.Cash)
	}
}

func TestBondSellReducesPosition(t *testing.T) {
	rows := []LedgerRow{
		lrow(2026, 11, 5, KindBuy, -3000, bond081.ISIN, 3),
		lrow(2026, 11, 8, KindSell, 1019.73, bond081.ISIN, 1),
	}
	st := ComputeState(rows, []Bond{bond081}, time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local))
	if st.Positions[0].Qty != 2 {
		t.Fatalf("qty = %v", st.Positions[0].Qty)
	}
}

func TestICUBondsAccountEndToEnd(t *testing.T) {
	s, _, _ := newS(t)
	acc := seedICU(t, s)
	b := bond081
	b.AccountID = acc.ID
	if _, err := s.SaveBond(b); err != nil {
		t.Fatal(err)
	}
	if bb, err := s.SaveBond(Bond{AccountID: acc.ID, ISIN: "UA4000239131", Maturity: "2029-03-21", CouponRate: 16}); err != nil || bb.Name != "ОВДП 21.03.2029" || bb.Nominal != 1000 {
		t.Fatalf("имя и номинал по умолчанию: %+v %v", bb, err)
	}
	if _, err := s.SaveBond(Bond{AccountID: acc.ID, ISIN: "X", Maturity: "bad", CouponRate: 1}); err == nil {
		t.Fatal("кривая дата погашения должна отклоняться")
	}
	if _, err := s.SaveLedgerRow(LedgerRow{AccountID: acc.ID, Time: ts(2026, 11, 5, 12), Kind: KindBuy, Amount: 3000}); err == nil {
		t.Fatal("покупка без облигации и количества должна отклоняться")
	}
	if _, err := s.SaveLedgerRow(LedgerRow{AccountID: acc.ID, Time: ts(2026, 11, 5, 12), Kind: KindDeposit, Amount: 5152.14}); err != nil {
		t.Fatal(err)
	}
	buy, err := s.SaveLedgerRow(LedgerRow{AccountID: acc.ID, Time: ts(2026, 11, 5, 12), Kind: KindBuy, Amount: 3070, ISIN: bond081.ISIN, Qty: 3})
	if err != nil || buy.Amount != -3070 {
		t.Fatalf("покупка: %+v %v", buy, err)
	}

	ov, err := s.ICUOverview(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Positions) != 1 || ov.Cash != 2082.14 {
		t.Fatalf("overview: cash=%v pos=%+v", ov.Cash, ov.Positions)
	}
	for _, r := range ov.Rows {
		if r.Kind == KindBuy && r.Status != "" {
			t.Fatalf("покупка не сверяется с кешем, статус %q", r.Status)
		}
	}
	accs, _ := s.GetAllAccounts()
	var bal float64
	for _, a := range accs {
		if a.ID == acc.ID {
			bal = a.Balance
		}
	}
	if bal != ov.Balance || bal <= 5152.14 {
		t.Fatalf("баланс счёта %v, overview %v", bal, ov.Balance)
	}
	txs, _ := s.GetAllTransactions(1000)
	for _, x := range txs {
		if x.AccountID == acc.ID && strings.Contains(x.Description, "Покупка") {
			t.Fatalf("покупка не должна попадать в поток операций: %+v", x)
		}
	}
	if err := s.DeleteBond(acc.ID, bond081.ISIN); err == nil {
		t.Fatal("облигацию с записями удалять нельзя")
	}
	if err := s.DeleteManualAccount(acc.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.ListBonds(acc.ID); len(l) != 0 {
		t.Fatal("облигации должны удаляться со счётом")
	}
}

func TestLedgerMigrationFromPhase1(t *testing.T) {
	s, _, _ := newS(t)
	_, _ = s.coreDB.Exec(`DROP TABLE icu_ledger`)
	if _, err := s.coreDB.Exec(`CREATE TABLE icu_ledger (id INTEGER PRIMARY KEY AUTOINCREMENT, account_id TEXT NOT NULL, time INTEGER NOT NULL, kind TEXT NOT NULL, amount REAL NOT NULL, note TEXT NOT NULL DEFAULT '', src_tx_id TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	if err := s.initManual(); err != nil {
		t.Fatal(err)
	}
	if err := s.initManual(); err != nil { // повторный запуск безопасен
		t.Fatal(err)
	}
	if !hasColumn(s.coreDB, "icu_ledger", "isin") || !hasColumn(s.coreDB, "icu_ledger", "qty") {
		t.Fatal("колонки isin/qty не добавлены")
	}
}

// Счёт ICU заводится сам, когда IBAN банка ICU (UA67 380946…) привязан к алиасу; руками не создаётся.
func TestICUAccountAutoCreatedFromAliasIBAN(t *testing.T) {
	s, _, _ := newS(t)
	const iban = "UA673809460000026009000000777"
	if accs, _ := s.GetAllAccounts(); len(accs) != 0 {
		t.Fatalf("до привязки счетов быть не должно: %+v", accs)
	}
	if err := s.LinkAccount("", icuAlias, iban, ""); err != nil {
		t.Fatal(err)
	}
	accs, _ := s.GetAllAccounts()
	var got *Account
	for i := range accs {
		if accs[i].Manual {
			got = &accs[i]
		}
	}
	if got == nil || got.BankKey != "icu" || got.IBAN != iban {
		t.Fatalf("счёт ICU не создан: %+v", accs)
	}
	if again, _ := s.GetAllAccounts(); len(again) != len(accs) {
		t.Fatal("повторный вызов не должен плодить счета")
	}
	// облигации по IBAN, ещё до привязки другого IBAN
	m, err := s.ICUAccountByIBAN(iban)
	if err != nil || m.ID != got.ID {
		t.Fatalf("по IBAN: %+v %v", m, err)
	}
	other, err := s.ICUAccountByIBAN("UA673809460000026009000000888")
	if err != nil || other.ID == m.ID {
		t.Fatalf("новый IBAN ICU должен создать свой счёт: %+v %v", other, err)
	}
	if _, err := s.ICUAccountByIBAN("UA583003350000000026001234567"); err == nil {
		t.Fatal("чужой банк счётом ICU не является")
	}
}

func TestLedgerSameDayBuyAboveDeposit(t *testing.T) {
	s, _, _ := newS(t)
	acc, _ := s.SaveManualAccount(ManualAccount{Name: "ICU", IBAN: icuIban})
	if _, err := s.SaveBond(Bond{AccountID: acc.ID, ISIN: "UA4000239131", Maturity: "2029-03-21", CouponRate: 16}); err != nil {
		t.Fatal(err)
	}
	// покупка внесена раньше пополнения, но в тот же день — всё равно выше пополнения
	if _, err := s.SaveLedgerRow(LedgerRow{AccountID: acc.ID, Time: ts(2026, 10, 23, 12), Kind: KindBuy, Amount: 2048.58, ISIN: "UA4000239131", Qty: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveLedgerRow(LedgerRow{AccountID: acc.ID, Time: ts(2026, 10, 23, 17), Kind: KindDeposit, Amount: 2048.58}); err != nil {
		t.Fatal(err)
	}
	ov, err := s.ICUOverview(acc.ID)
	if err != nil || len(ov.Rows) != 2 || ov.Rows[0].Kind != KindBuy || ov.Rows[1].Kind != KindDeposit {
		t.Fatalf("порядок: %+v %v", ov.Rows, err)
	}
}

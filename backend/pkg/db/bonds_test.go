package db

import (
	"economic/pkg/stmtkit"
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
	if want := stmtkit.Round2(130 + 7*246.6 + 3000); st.Cash != want || st.Balance != want {
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

// Облигации без условий в справочнике не теряются: оцениваются по средней цене покупки.
func TestBondWithoutTermsIsValuedAtCost(t *testing.T) {
	rows := []LedgerRow{
		lrow(2026, 11, 5, KindDeposit, 3200, "", 0),
		lrow(2026, 11, 5, KindBuy, -3070, bond081.ISIN, 3),
	}
	st := ComputeState(rows, nil, time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local))
	if len(st.Positions) != 1 || !st.Positions[0].NoTerms || st.Positions[0].Value != 3070 {
		t.Fatalf("позиция без условий: %+v", st.Positions)
	}
	if st.Cash != 130 || st.Balance != 3200 {
		t.Fatalf("свободные %v баланс %v", st.Cash, st.Balance)
	}
	// продали штуку — остаток по средней цене
	rows = append(rows, lrow(2026, 11, 8, KindSell, 1019.73, bond081.ISIN, 1))
	st = ComputeState(rows, nil, time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local))
	if st.Positions[0].Qty != 2 || st.Positions[0].Value != stmtkit.Round2(2*3070.0/3) {
		t.Fatalf("после продажи: %+v", st.Positions[0])
	}
}

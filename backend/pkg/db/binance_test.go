package db

import (
	"testing"
	"time"
)

func TestBinanceSnapshotsAccountAndSyntheticTx(t *testing.T) {
	s, _, _ := newS(t)
	if accs, _ := s.GetAllAccounts(); len(accs) != 0 {
		t.Fatalf("до первого снимка счёта Binance нет: %+v", accs)
	}
	d1 := time.Date(2026, 11, 8, 20, 0, 0, 0, time.Local)
	d2 := time.Date(2026, 11, 9, 9, 0, 0, 0, time.Local)
	d2b := time.Date(2026, 11, 9, 22, 0, 0, 0, time.Local)
	for _, x := range []struct {
		at    time.Time
		total float64
	}{{d1, 1000}, {d2, 1200}, {d2b, 1150}} { // второй снимок за 9-е заменяет первый
		if err := s.SaveBinanceSnapshot(x.total, 41.5, map[string]any{"items": []string{}}, x.at); err != nil {
			t.Fatal(err)
		}
	}
	accs, _ := s.GetAllAccounts()
	if len(accs) != 1 || accs[0].ID != BinanceAccountID || accs[0].Balance != 1150 || accs[0].BankKey != "binance" || !accs[0].HasDetail || accs[0].Currency != 980 {
		t.Fatalf("счёт: %+v", accs)
	}
	txs, _ := s.GetAllTransactions(100)
	var sum float64
	n := 0
	for _, x := range txs {
		if x.AccountID != BinanceAccountID {
			continue
		}
		n++
		sum += x.Amount
		if !x.Internal {
			t.Fatalf("оценка — внутреннее движение, не доход/расход: %+v", x)
		}
	}
	// 8-го: +1000 (подключено), 9-го: +150 (1150−1000); сумма дельт = баланс
	if n != 2 || sum != 1150 {
		t.Fatalf("операции: n=%d sum=%v %+v", n, sum, txs)
	}
	ov, _ := s.BinanceOverview(true)
	if ov.Days != 2 || ov.Snapshot == nil || ov.Snapshot.Total != 1150 {
		t.Fatalf("overview: %+v", ov)
	}
}

func TestBinanceTxDoesNotBreakOtherTransactions(t *testing.T) {
	s, _, _ := newS(t)
	fillCache(t, s)
	before, _ := s.GetAllTransactions(1000)
	_ = s.SaveBinanceSnapshot(500, 40, nil, time.Now())
	after, _ := s.GetAllTransactions(1000)
	if len(after) != len(before)+1 {
		t.Fatalf("ожидалась одна синтетическая операция: %d → %d", len(before), len(after))
	}
}

func TestBinanceMinutesCandlesAndTxSwitch(t *testing.T) {
	s, _, _ := newS(t)
	_ = s.SaveBinanceSnapshot(500, 40, nil, time.Now())
	if txs, _ := s.GetAllTransactions(100); len(txs) != 1 {
		t.Fatalf("без ряда есть синтетическая операция: %+v", txs)
	}
	// ряд: минуты 0..9 от t0, значения 100,110,90,95,... ; ряд начинается с t0
	t0 := int64(1_700_000_000) / 60 * 60
	vals := []float64{100, 110, 90, 95, 95, 120, 118, 118, 118, 130}
	if err := s.ReplaceBinanceMinutes(t0, vals); err != nil {
		t.Fatal(err)
	}
	if txs, _ := s.GetAllTransactions(100); len(txs) != 0 {
		t.Fatalf("с рядом синтетические операции отключены: %+v", txs)
	}
	// корзины: до ряда | [t0,t0+5m) | [t0+5m,t0+10m) | после ряда (плоская)
	starts := []int64{t0 - 600, t0, t0 + 300, t0 + 600}
	got, has, err := s.BinanceCandles(starts, t0+900)
	if err != nil {
		t.Fatal(err)
	}
	if has[0] {
		t.Fatalf("до ряда свечи нет: %v", got[0])
	}
	// первая корзина: open = первая точка (предыдущей нет), high 110, low 90, close 95
	if got[1] != [4]float64{100, 110, 90, 95} {
		t.Fatalf("корзина 1: %v", got[1])
	}
	// вторая: open = close предыдущей (95), high 130, low 95, close 130
	if got[2] != [4]float64{95, 130, 95, 130} {
		t.Fatalf("корзина 2: %v", got[2])
	}
	if !has[3] || got[3] != [4]float64{130, 130, 130, 130} {
		t.Fatalf("после ряда — плоская: %v", got[3])
	}
	// дописывание
	if err := s.AppendBinanceMinutes(t0+600, []float64{140}); err != nil {
		t.Fatal(err)
	}
	if f, l, n := s.BinanceSeriesInfo(); f != t0 || l != t0+600 || n != 11 {
		t.Fatalf("info: %d %d %d", f, l, n)
	}
	accs, _ := s.GetAllAccounts()
	if len(accs) != 1 || accs[0].OpenedAt != t0 {
		t.Fatalf("счёт открыт с начала ряда: %+v", accs)
	}
}

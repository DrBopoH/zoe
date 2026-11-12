package db

import "testing"

func TestBinanceLivesInCacheOnly(t *testing.T) {
	s, _, _ := newS(t)
	if accs, _ := s.GetAllAccounts(); len(accs) != 0 {
		t.Fatalf("до первого опроса счёта Binance нет: %+v", accs)
	}
	t0 := int64(1_700_000_000) / 60 * 60
	if err := s.SaveBinance(BinanceSave{Portfolio: 1150, Start: t0, Vals: []float64{100, 110, 90}, Replace: true, Detail: map[string]any{"total": 1150}}); err != nil {
		t.Fatal(err)
	}
	accs, _ := s.GetAllAccounts()
	if len(accs) != 1 {
		t.Fatalf("счета: %+v", accs)
	}
	a := accs[0]
	if a.ID != BinanceAccountID || a.Type != BinanceAccountType || a.Balance != 1150 || a.BankKey != "binance" || !a.HasDetail || a.Currency != 980 || a.OpenedAt != t0 {
		t.Fatalf("счёт: %+v", a)
	}
	if txs, _ := s.GetAllTransactions(100); len(txs) != 0 {
		t.Fatalf("синтетических операций нет: %+v", txs)
	}
	if len(s.BinanceDetail()) == 0 {
		t.Fatal("разбор не сохранён")
	}
	// в core.db от Binance нет ничего
	for _, n := range legacyBinanceCore {
		if tableExists(s.coreDB, n) {
			t.Fatalf("в core.db таблица %s", n)
		}
	}
	// повторный опрос обновляет баланс, а не плодит счета
	_ = s.SaveBinance(BinanceSave{Portfolio: 900, Start: t0 + 180, Vals: []float64{95}, Detail: map[string]any{"total": 900}})
	if accs, _ := s.GetAllAccounts(); len(accs) != 1 || accs[0].Balance != 900 {
		t.Fatalf("повторно: %+v", accs)
	}
	// вайп кеша сносит всё облачное
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if _, _, n := s.BinanceSeriesInfo(); n != 0 || len(s.BinanceDetail()) != 0 {
		t.Fatalf("после вайпа остались данные: n=%d", n)
	}
	if accs, _ := s.GetAllAccounts(); len(accs) != 0 {
		t.Fatalf("после вайпа остался счёт: %+v", accs)
	}
}

func TestLegacyBinanceTablesDroppedFromCore(t *testing.T) {
	s, _, _ := newS(t)
	for _, n := range legacyBinanceCore {
		if _, err := s.coreDB.Exec(`CREATE TABLE ` + n + ` (x INTEGER)`); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.dropLegacyBinanceCore(); err != nil {
		t.Fatal(err)
	}
	for _, n := range legacyBinanceCore {
		if tableExists(s.coreDB, n) {
			t.Fatalf("%s осталась в core.db", n)
		}
	}
}

func TestBinanceMinutesCandles(t *testing.T) {
	s, _, _ := newS(t)
	t0 := int64(1_700_000_000) / 60 * 60
	vals := []float64{100, 110, 90, 95, 95, 120, 118, 118, 118, 130}
	// Origin = первая точка: как ряд без истории движений
	if err := s.SaveBinance(BinanceSave{Portfolio: 130, Start: t0, Vals: vals, Replace: true, Origin: 100}); err != nil {
		t.Fatal(err)
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
	if got[1] != [4]float64{100, 110, 90, 95} {
		t.Fatalf("корзина 1: %v", got[1])
	}
	if got[2] != [4]float64{95, 130, 95, 130} {
		t.Fatalf("корзина 2: %v", got[2])
	}
	if !has[3] || got[3] != [4]float64{130, 130, 130, 130} {
		t.Fatalf("после ряда — плоская: %v", got[3])
	}
	if err := s.SaveBinance(BinanceSave{Portfolio: 140, Start: t0 + 600, Vals: []float64{140}}); err != nil {
		t.Fatal(err)
	}
	if f, l, n := s.BinanceSeriesInfo(); f != t0 || l != t0+600 || n != 11 {
		t.Fatalf("info: %d %d %d", f, l, n)
	}
}

// Каждый просмотренный период сразу в кеше: движения + покрытие; повторная запись не дублирует.
func TestBinanceWindowsCachedAndResumable(t *testing.T) {
	s, _, _ := newS(t)
	if s.BinanceCovered("депозиты", "", 100, 200) {
		t.Fatal("пустой кеш ничего не покрывает")
	}
	fl := []BinanceFlow{{T: 150, Asset: "USDT", Delta: 100, Kind: "deposit"}, {T: 150, Asset: "USDT", Delta: 100, Kind: "deposit"}}
	if err := s.SaveBinanceWindow("депозиты", "", 100, 200, fl); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBinanceWindow("депозиты", "", 100, 200, fl); err != nil { // тот же период ещё раз
		t.Fatal(err)
	}
	// хвост заново запрошен шире: два одинаковых движения те же, третье новое
	if err := s.SaveBinanceWindow("депозиты", "", 100, 300, append(fl, BinanceFlow{T: 250, Asset: "BTC", Delta: 1, Kind: "deposit"})); err != nil {
		t.Fatal(err)
	}
	got, _ := s.BinanceFlows()
	if len(got) != 3 {
		t.Fatalf("ожидали 3 движения без дублей: %+v", got)
	}
	if !s.BinanceCovered("депозиты", "", 100, 200) || !s.BinanceCovered("депозиты", "", 120, 180) || s.BinanceCovered("депозиты", "", 100, 400) || s.BinanceCovered("выводы", "", 100, 200) {
		t.Fatal("покрытие определяется неверно")
	}
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.BinanceFlows(); len(got) != 0 || s.BinanceCovered("депозиты", "", 100, 200) {
		t.Fatal("вайп кеша сносит и движения, и покрытие")
	}
}

// Счёт попадает в кеш сразу (до истории), операции счёта пишутся по мере оценки движений и переживают повторную сборку;
// первая свеча ряда открывается с origin, поэтому скачок депозита виден.
func TestBinanceAccountAndTxsAreCachedEarly(t *testing.T) {
	s, _, _ := newS(t)
	t0 := int64(1_700_000_000) / 60 * 60
	if err := s.SaveBinanceAccount(500, map[string]any{"total": 500}); err != nil {
		t.Fatal(err)
	}
	accs, _ := s.GetAccounts()
	if len(accs) != 1 || accs[0].ID != BinanceAccountID || accs[0].Balance != 500 {
		t.Fatalf("счёт должен быть в кеше до истории: %+v", accs)
	}
	if err := s.SaveBinanceWindow("депозиты", "", t0*1000-1000, t0*1000+1000, []BinanceFlow{{T: t0 * 1000, Asset: "USDT", Delta: 10, Kind: "deposit"}, {T: t0*1000 + 5, Asset: "USDT", Delta: 3, Kind: "trade"}}); err != nil {
		t.Fatal(err)
	}
	un, err := s.BinanceUnvaluedFlows([]string{"deposit"})
	if err != nil || len(un) != 1 || un[0].Kind != "deposit" {
		t.Fatalf("без оценки только депозит: %+v %v", un, err)
	}
	if err := s.SaveBinanceTxs([]BinanceTx{{FlowID: un[0].ID, T: un[0].T, Desc: "Binance: депозит 10 USDT", UAH: 400}}); err != nil {
		t.Fatal(err)
	}
	if un, _ := s.BinanceUnvaluedFlows([]string{"deposit"}); len(un) != 0 {
		t.Fatalf("после записи оценивать нечего: %+v", un)
	}
	txs, _ := s.GetAllTransactions(100)
	if len(txs) != 1 || txs[0].AccountID != BinanceAccountID || txs[0].Amount != 400 || !txs[0].Internal || txs[0].Description != "Binance: депозит 10 USDT" {
		t.Fatalf("операции: %+v", txs)
	}
	if err := s.SaveBinance(BinanceSave{Portfolio: 500, Start: t0, Vals: []float64{400, 410, 500}, Replace: true}); err != nil {
		t.Fatal(err)
	}
	if txs, _ := s.GetAllTransactions(100); len(txs) != 1 {
		t.Fatalf("сборка ряда не должна терять операции: %+v", txs)
	}
	if accs, _ := s.GetAccounts(); accs[0].OpenedAt != t0 {
		t.Fatalf("opened_at = первая минута ряда: %+v", accs[0])
	}
	c, has, _ := s.BinanceCandles([]int64{t0}, t0+300)
	if !has[0] || c[0][0] != 0 || c[0][3] != 500 || c[0][1] != 500 { // открытие 0, закрытие 500: скачок виден
		t.Fatalf("свеча: %v", c[0])
	}
}

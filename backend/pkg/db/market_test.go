package db

import (
	"path/filepath"
	"testing"
	"time"

	"economic/pkg/market"
)

func TestMarketCompaniesSnapshotReplacesAndSearches(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first := []market.Company{
		{Ticker: "tsla", Name: "Tesla, Inc.", Exchange: "Nasdaq", CIK: 1318605, MarketCap: 1.3e12, Price: 420.5, IPOYear: 2010, Sector: "Consumer Discretionary"},
		{Ticker: "AAPL", Name: "Apple Inc.", Exchange: "Nasdaq", CIK: 320193, MarketCap: 4.5e12, Price: 300},
		{Ticker: "OLD", Name: "Old Corp", CIK: 1, MarketCap: 1e6, Price: 1},
	}
	if err := s.SaveMarketCompanies(first, 1); err != nil {
		t.Fatal(err)
	}
	all, err := s.MarketCompanies("", 0)
	if err != nil || len(all) != 3 || all[0].Ticker != "AAPL" || all[1].Ticker != "TSLA" || all[1].CIK != 1318605 || all[1].IPOYear != 2010 {
		t.Fatalf("крупные первыми, тикер в верхнем регистре: %v %+v", err, all)
	}
	if got, _ := s.MarketCompanies("tes", 0); len(got) != 1 || got[0].Ticker != "TSLA" {
		t.Fatalf("поиск по названию без регистра: %+v", got)
	}
	if got, _ := s.MarketCompanies("", 2); len(got) != 2 {
		t.Fatalf("limit: %+v", got)
	}
	// новый снимок заменяет прежний целиком; пустой — ничего не меняет
	if err := s.SaveMarketCompanies([]market.Company{{Ticker: "AAPL", Name: "Apple Inc.", CIK: 320193, MarketCap: 5e12, Price: 310}}, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMarketCompanies(nil, 3); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.MarketCompanies("", 0); len(got) != 1 || got[0].MarketCap != 5e12 {
		t.Fatalf("замена снимка: %+v", got)
	}
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.MarketCompanies("", 0); len(got) != 0 {
		t.Fatalf("«стереть кеш» уносит и список компаний: %+v", got)
	}
}

func TestStockPricesReplaceAndPickStaleTop(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SaveMarketCompanies([]market.Company{
		{Ticker: "AAPL", Name: "Apple", CIK: 1, MarketCap: 4e12, Price: 1},
		{Ticker: "NVDA", Name: "Nvidia", CIK: 2, MarketCap: 3e12, Price: 1},
		{Ticker: "TSLA", Name: "Tesla", CIK: 3, MarketCap: 1e12, Price: 1},
	}, 1); err != nil {
		t.Fatal(err)
	}
	now := int64(1_000_000)
	// до истории — все из топ-2 просрочены, крупные первыми
	if got, err := s.StockTickersToFetch(2, 20*time.Hour, now); err != nil || len(got) != 2 || got[0] != "AAPL" || got[1] != "NVDA" {
		t.Fatalf("топ-2 без истории: %v %v", err, got)
	}
	if err := s.SaveStockPrices("aapl", []market.PriceBar{{Date: "2026-09-01", Close: 100, Adj: 99}, {Date: "2026-10-01", Close: 110, Adj: 109}}, []market.Split{{Date: "2026-09-15", Ratio: 4}}, now-3600); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.StockTickersToFetch(2, 20*time.Hour, now); len(got) != 1 || got[0] != "NVDA" {
		t.Fatalf("свежая история AAPL не перекачивается: %v", got)
	}
	if sp, err := s.StockSplits("aapl"); err != nil || len(sp) != 1 || sp[0].Ratio != 4 || sp[0].Date != "2026-09-15" {
		t.Fatalf("сплиты тикера: %v %+v", err, sp)
	}
	if got, _ := s.StockTickersToFetch(3, 20*time.Hour, now+30*3600); len(got) != 3 || got[0] != "AAPL" {
		t.Fatalf("через сутки с лишним снова всё: %v", got)
	}
	// новая история заменяет прежнюю целиком (сплит пересчитал прошлые цены)
	if err := s.SaveStockPrices("AAPL", []market.PriceBar{{Date: "2026-10-01", Close: 55, Adj: 54.5}}, nil, now); err != nil {
		t.Fatal(err)
	}
	if sp, _ := s.StockSplits("AAPL"); len(sp) != 0 {
		t.Fatalf("сплиты заменяются вместе с историей: %+v", sp)
	}
	bars, at, err := s.StockPrices("aapl")
	if err != nil || at != now || len(bars) != 1 || bars[0].Close != 55 || bars[0].Adj != 54.5 || bars[0].Ticker != "AAPL" {
		t.Fatalf("замена истории: %v %d %+v", err, at, bars)
	}
	if bars, at, _ = s.StockPrices("NONE"); len(bars) != 0 || at != 0 {
		t.Fatalf("нет истории: %+v %d", bars, at)
	}
	if err := s.SaveStockPrices("AAPL", nil, nil, now); err != nil {
		t.Fatal(err)
	}
	if c, ok, err := s.MarketCompany("nvda"); err != nil || !ok || c.CIK != 2 {
		t.Fatalf("компания по тикеру без учёта регистра: %v %v %+v", err, ok, c)
	}
	if _, ok, _ := s.MarketCompany("ZZZ"); ok {
		t.Fatal("чужого тикера нет")
	}
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if bars, at, _ = s.StockPrices("AAPL"); len(bars) != 0 || at != 0 {
		t.Fatalf("«стереть кеш» уносит и историю: %+v", bars)
	}
}

// История, скачанная до сплитов (в кеше есть цены, но нет отметки о сплитах), перекачивается один раз.
func TestStockTickersToFetchRefetchesHistoryWithoutSplitMark(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SaveMarketCompanies([]market.Company{{Ticker: "AAPL", Name: "Apple", CIK: 1, MarketCap: 4e12, Price: 1}}, 1); err != nil {
		t.Fatal(err)
	}
	now := int64(1_000_000)
	if _, err := s.cacheDB.Exec(`INSERT INTO stock_fetched(ticker, at) VALUES('AAPL', ?)`, now); err != nil { // как после версии без сплитов
		t.Fatal(err)
	}
	if got, _ := s.StockTickersToFetch(10, 20*time.Hour, now); len(got) != 1 {
		t.Fatalf("свежая история без отметки о сплитах качается снова: %v", got)
	}
	if err := s.SaveStockPrices("AAPL", []market.PriceBar{{Date: "2026-10-01", Close: 1, Adj: 1}}, nil, now); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.StockTickersToFetch(10, 20*time.Hour, now); len(got) != 0 {
		t.Fatalf("после загрузки (даже без единого сплита) отметка есть: %v", got)
	}
}

func TestStockFramesStoreSharesAndTodo(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	all := []market.FrameRequest{
		{Tag: market.TagSharesBalance, Unit: "shares", Period: "CY2025Q2I"},
		{Tag: market.TagSharesBalance, Unit: "shares", Period: "CY2025Q3I", Recent: true},
		{Tag: market.TagSharesCover, Unit: "shares", Period: "CY2025Q3I", Recent: true},
	}
	now := int64(10_000_000)
	if todo, err := s.StockFramesTodo(all, 72*time.Hour, now); err != nil || len(todo) != 3 {
		t.Fatalf("ничего не скачано: %v %v", err, todo)
	}
	if err := s.SaveStockFrame(all[0], []market.FrameFact{{CIK: 320193, End: "2025-06-28", Val: 14.9e9}, {CIK: 5, End: "2025-06-30", Val: 7}}, now-10); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveStockFrame(all[1], []market.FrameFact{{CIK: 320193, End: "2025-09-27", Val: 14.8e9}}, now-100*3600); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveStockFrame(all[2], nil, now-10); err != nil { // период без значений тоже отмечается: заново не качаем
		t.Fatal(err)
	}
	// старый кадр не перекачивается, а Recent старше 72 часов — да
	if todo, _ := s.StockFramesTodo(all, 72*time.Hour, now); len(todo) != 1 || todo[0].Key() != all[1].Key() {
		t.Fatalf("только устаревший свежий кадр: %v", todo)
	}
	sh, err := s.StockShares(320193)
	if err != nil || len(sh) != 2 || sh[0].End != "2025-06-28" || sh[1].Tag != market.TagSharesBalance || sh[1].Val != 14.8e9 {
		t.Fatalf("акции Apple по возрастанию даты: %v %+v", err, sh)
	}
	// перекачанный кадр заменяет прежний целиком
	if err := s.SaveStockFrame(all[1], []market.FrameFact{{CIK: 1, End: "2025-09-30", Val: 3}}, now); err != nil {
		t.Fatal(err)
	}
	if sh, _ = s.StockShares(320193); len(sh) != 1 {
		t.Fatalf("замена кадра: %+v", sh)
	}
	if todo, _ := s.StockFramesTodo(all, 72*time.Hour, now); len(todo) != 0 {
		t.Fatalf("всё свежее: %v", todo)
	}
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if sh, _ = s.StockShares(1); len(sh) != 0 {
		t.Fatalf("«стереть кеш» уносит и отчётность: %+v", sh)
	}
}

func TestMarketBondsMergeKeepsKnownAndFeedsICUTerms(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// реестр (условия), потом аукцион (доходность без ставки) — ставка и дата не должны потеряться
	if err := s.SaveMarketBonds([]market.Security{
		{Source: "nbu", ID: "ua4000239081", Name: "ОВДП 2030", Currency: "UAH", Maturity: "2030-02-20", CouponRate: 16.44, Nominal: 1000},
		{Source: "nbu", ID: "UA4000000777", Name: "USD", Currency: "USD", Maturity: "2030-01-01", CouponRate: 4, Nominal: 1000},
		{Source: "ust", ID: "912810XX1", Name: "30-Year Bond", Currency: "USD", Maturity: "2056-11-15", CouponRate: 4.625, Nominal: 100},
	}, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMarketBonds([]market.Security{{Source: "nbu", ID: "UA4000239081", Yield: 16.1, LastAuction: "2026-11-12"}}, 2); err != nil {
		t.Fatal(err)
	}
	nbu, err := s.MarketBonds("nbu")
	if err != nil || len(nbu) != 2 {
		t.Fatalf("nbu: %v %+v", err, nbu)
	}
	var a market.Security
	for _, b := range nbu {
		if b.ID == "UA4000239081" {
			a = b
		}
	}
	if a.CouponRate != 16.44 || a.Maturity != "2030-02-20" || a.Yield != 16.1 || a.LastAuction != "2026-11-12" || a.Name != "ОВДП 2030" {
		t.Fatalf("пустое не должно затирать известное: %+v", a)
	}
	if all, _ := s.MarketBonds(""); len(all) != 3 {
		t.Fatalf("все источники: %d", len(all))
	}

	// условия для расчёта: скачанная гривневая бумага подходит, USD и US Treasury — нет
	terms, err := s.bondTerms()
	if err != nil || len(terms) != 1 || terms[0].ISIN != "UA4000239081" || terms[0].CouponRate != 16.44 {
		t.Fatalf("bondTerms: %v %+v", err, terms)
	}
	// своя запись главнее скачанной
	if _, err := s.SaveBond(Bond{ISIN: "UA4000239081", Maturity: "2030-02-20", CouponRate: 10, Nominal: 1000}); err != nil {
		t.Fatal(err)
	}
	terms, _ = s.bondTerms()
	if len(terms) != 1 || terms[0].CouponRate != 10 {
		t.Fatalf("ручной справочник должен перекрывать скачанный: %+v", terms)
	}
	// справочник для редактора (ListBonds) скачанным не засоряется
	if own, _ := s.ListBonds(); len(own) != 1 {
		t.Fatalf("ListBonds только свои: %+v", own)
	}
	// ICU считает купоны по скачанным условиям, если своих нет
	if err := s.DeleteBond("UA4000239081"); err != nil {
		t.Fatal(err)
	}
	terms, _ = s.bondTerms()
	rows := []LedgerRow{
		lrow(2026, 11, 5, KindDeposit, 3200, "", 0),
		lrow(2026, 11, 5, KindBuy, -3070, "UA4000239081", 3),
	}
	st := ComputeState(rows, terms, time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local))
	if len(st.Positions) != 1 || st.Positions[0].NoTerms || st.Positions[0].Accrued != 34.33 {
		t.Fatalf("скачанные условия должны дать НКД как у ручных: %+v", st.Positions)
	}

	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if all, _ := s.MarketBonds(""); len(all) != 0 {
		t.Fatalf("вайп кеша должен стирать и рыночные данные: %d", len(all))
	}
}

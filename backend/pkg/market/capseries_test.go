package market

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCapSeriesAdjustsSharesForLaterSplits(t *testing.T) {
	// Apple-подобный случай: на 2020-07-17 в отчёте 4,28 млрд акций, сплит 4:1 в августе 2020; цены Yahoo уже поправлены на сплит
	bars := []PriceBar{
		{Ticker: "AAPL", Date: "2020-07-01", Close: 91},
		{Ticker: "AAPL", Date: "2020-09-01", Close: 115},
		{Ticker: "AAPL", Date: "2020-10-01", Close: 110},
	}
	splits := []Split{{Date: "2020-08-31", Ratio: 4}}
	shares := []Fact{
		{Tag: TagSharesCover, Period: "CY2020Q2I", End: "2020-07-17", Val: 4_275_634_000},
		{Tag: TagSharesBalance, Period: "CY2020Q3I", End: "2020-09-26", Val: 17_000_000_000}, // после сплита: уже новая база
	}
	got := CapSeries(bars, splits, shares)
	if len(got) != 3 {
		t.Fatalf("три месяца: %+v", got)
	}
	// июль: 4,28 млрд × 4 (сплит после отчёта) × 91 ≈ 1,556 трлн
	if want := 91 * 4_275_634_000.0 * 4; got[0].Cap != want {
		t.Fatalf("июль: %v, ждали %v", got[0].Cap, want)
	}
	// сентябрь: отчёт на 26 сентября уже есть (после сплита: уже новая база), его и берём
	if want := 115 * 17_000_000_000.0; got[1].Cap != want {
		t.Fatalf("сентябрь: %v, ждали %v", got[1].Cap, want)
	}
	// октябрь: тот же сентябрьский отчёт
	if want := 110 * 17_000_000_000.0; got[2].Cap != want {
		t.Fatalf("октябрь: %v, ждали %v", got[2].Cap, want)
	}
}

func TestCapSeriesSkipsMonthsWithoutFreshShares(t *testing.T) {
	bars := []PriceBar{{Date: "2015-01-01", Close: 10}, {Date: "2020-01-01", Close: 10}, {Date: "2021-12-01", Close: 10}, {Date: "кривая", Close: 10}}
	shares := []Fact{{Tag: TagSharesBalance, Period: "CY2019Q4I", End: "2019-12-31", Val: 100}}
	got := CapSeries(bars, nil, shares)
	if len(got) != 1 || got[0].Date != "2020-01-01" || got[0].Cap != 1000 {
		t.Fatalf("до первого отчёта и спустя 15 месяцев после последнего — пробел: %+v", got)
	}
	if CapSeries(bars, nil, nil) != nil {
		t.Fatal("нет акций — нет рядов")
	}
}

func TestCapSeriesPrefersBalanceOverCoverAndReverseSplit(t *testing.T) {
	bars := []PriceBar{{Date: "2024-02-01", Close: 50}}
	shares := []Fact{
		{Tag: TagSharesCover, Period: "CY2023Q4I", End: "2024-01-20", Val: 999},
		{Tag: TagSharesBalance, Period: "CY2023Q4I", End: "2023-12-31", Val: 200},
		{Tag: TagSharesBalance, Period: "CY2022Q4I", End: "2022-12-31", Val: 0}, // мусор
	}
	// обратный сплит 1:10 после отчёта: акций стало в десять раз меньше, цена Yahoo поправлена вверх
	got := CapSeries(bars, []Split{{Date: "2024-01-15", Ratio: 0.1}}, shares)
	if len(got) != 1 || got[0].Cap != 50*200*0.1 {
		t.Fatalf("баланс главнее обложки, обратный сплит: %+v", got)
	}
}

func TestStockFrameRequestsCoverFinishedQuarters(t *testing.T) {
	all := StockFrameRequests(time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	var reqs, equity, annual []FrameRequest
	for _, r := range all {
		switch {
		case r.Unit == "shares":
			reqs = append(reqs, r)
		case strings.HasSuffix(r.Period, "I"):
			equity = append(equity, r)
		default:
			annual = append(annual, r)
		}
	}
	// с 2009Q4 по 2026Q3 (Q3 закончился 30.09, 45 дней на отчёты прошли 14.11), два тега на период
	first, last := reqs[0], reqs[len(reqs)-1]
	if first.Period != "CY2009Q4I" || last.Period != "CY2026Q3I" || len(reqs) != 2*(1+16*4+3) {
		t.Fatalf("периоды: %s … %s, всего %d", first.Period, last.Period, len(reqs))
	}
	if first.Tag != TagSharesBalance || reqs[1].Tag != TagSharesCover || first.Unit != "shares" || first.Key() != TagSharesBalance+"|CY2009Q4I" {
		t.Fatalf("теги: %+v %+v", first, reqs[1])
	}
	// капитал и активы — те же кварталы, в долларах
	if len(equity) != len(reqs) || equity[0].Tag != TagEquity || equity[1].Tag != TagAssets || equity[0].Unit != "USD" {
		t.Fatalf("капитал и активы: %d кадров, начало %+v", len(equity), equity[:2])
	}
	// годовые: цепочка выручки и прибыль за 2009–2025 (2025 закрыт 31.12, 90 дней на годовые отчёты прошли 31.03.2026)
	if want := 17 * (len(RevenueChain) + 1); len(annual) != want || annual[0].Period != "CY2009" || annual[len(annual)-1].Period != "CY2025" || annual[len(annual)-1].Tag != TagNetIncome {
		t.Fatalf("годовых кадров %d, ждали %d: %+v", len(annual), want, annual[len(annual)-1])
	}
	recent := 0
	for _, r := range all {
		if r.Recent {
			recent++
			if r.Period != "CY2026Q2I" && r.Period != "CY2026Q3I" && r.Period != "CY2024" && r.Period != "CY2025" {
				t.Fatalf("свежие — два последних периода каждого вида: %+v", r)
			}
		}
	}
	if recent != 2*2+2*2+2*(len(RevenueChain)+1) {
		t.Fatalf("свежих кадров %d", recent)
	}
}

func TestBuildFundamentalsChainsRevenueAndKeepsLosses(t *testing.T) {
	facts := []Fact{
		{Tag: TagRevenueSales, Period: "CY2016", End: "2016-12-31", Val: 7e9},
		{Tag: TagRevenueContracts, Period: "CY2023", End: "2023-09-30", Val: 383e9},
		{Tag: TagRevenues, Period: "CY2023", End: "2023-09-30", Val: 384e9}, // Revenues главнее
		{Tag: TagNetIncome, Period: "CY2023", End: "2023-09-30", Val: -5e9},
		{Tag: TagEquity, Period: "CY2023Q4I", End: "2023-12-30", Val: 74e9},
		{Tag: TagEquity, Period: "CY2023", End: "2023-12-31", Val: 1}, // годовой период у «капитала» — не точка на дату
		{Tag: TagAssets, Period: "CY2023Q4I", End: "2023-12-30", Val: 353e9},
		{Tag: TagSharesBalance, Period: "CY2023Q4I", End: "2023-12-30", Val: 15e9}, // чужой тег
	}
	f := BuildFundamentals(facts)
	if len(f.Revenue) != 2 || f.Revenue[0].Date != "2016-12-31" || f.Revenue[1].Val != 384e9 {
		t.Fatalf("выручка: %+v", f.Revenue)
	}
	if len(f.NetIncome) != 1 || f.NetIncome[0].Val != -5e9 {
		t.Fatalf("убыток потерян: %+v", f.NetIncome)
	}
	if len(f.Equity) != 1 || f.Equity[0].Val != 74e9 || len(f.Assets) != 1 || f.Assets[0].Date != "2023-12-30" {
		t.Fatalf("капитал и активы: %+v %+v", f.Equity, f.Assets)
	}
	if e := BuildFundamentals(nil); e.Revenue == nil || e.Assets == nil {
		t.Fatalf("пустая отчётность должна отдавать пустые списки, не null: %+v", e)
	}
}

func framesClient(t *testing.T, missing string) (*Client, *[]string) {
	t.Helper()
	asked := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != secUserAgent {
			http.Error(w, "нужен User-Agent SEC", http.StatusForbidden)
			return
		}
		asked = append(asked, r.URL.Path)
		if missing != "" && strings.Contains(r.URL.Path, missing) {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"taxonomy":"us-gaap","pts":3,"data":[{"cik":320193,"entityName":"Apple","end":"2024-12-28","val":15040731000},{"cik":1318605,"end":"2024-12-31","val":3216000000},{"cik":5,"end":"","val":1},{"cik":6,"end":"2024-12-31","val":0}]}`)
	}))
	t.Cleanup(srv.Close)
	c := NewClient()
	c.EDGARBase = srv.URL
	c.Retries = 0
	return c, &asked
}

func TestStockFrameParsesFacts(t *testing.T) {
	c, asked := framesClient(t, "")
	facts, err := c.StockFrame(context.Background(), FrameRequest{Tag: TagSharesBalance, Unit: "shares", Period: "CY2024Q4I"})
	if err != nil || len(facts) != 2 || facts[0].CIK != 320193 || facts[0].End != "2024-12-28" || facts[0].Val != 15040731000 {
		t.Fatalf("кадр: %v %+v", err, facts)
	}
	if (*asked)[0] != "/api/xbrl/frames/us-gaap/CommonStockSharesOutstanding/shares/CY2024Q4I.json" {
		t.Fatalf("адрес кадра: %v", *asked)
	}
	c.EDGARBase = ""
	if _, err := c.StockFrame(context.Background(), FrameRequest{}); err == nil {
		t.Fatal("пустой адрес отключает опрос")
	}
}

func TestStockFactsSavesEachFrameAndToleratesMissing(t *testing.T) {
	c, _ := framesClient(t, "CY2020Q2I")
	todo := []FrameRequest{{Tag: TagSharesBalance, Unit: "shares", Period: "CY2020Q1I"}, {Tag: TagSharesBalance, Unit: "shares", Period: "CY2020Q2I"}, {Tag: TagSharesCover, Unit: "shares", Period: "CY2020Q3I"}}
	saved := map[string]int{}
	res := c.StockFacts(context.Background(), nil, todo, func(r FrameRequest, f []FrameFact) error { saved[r.Key()] = len(f); return nil })
	if res.Source != SourceStockFacts || res.Fetched != 4 || res.Skipped != 1 || res.Error != "" || len(saved) != 2 {
		t.Fatalf("один кадр из трёх без значений — не ошибка источника: %+v %v", res, saved)
	}
	res = c.StockFacts(context.Background(), nil, todo[1:2], func(FrameRequest, []FrameFact) error { return nil })
	if !strings.HasPrefix(res.Error, "без значений 1 из 1 кадров") {
		t.Fatalf("половина кадров и больше без значений — ошибка: %+v", res)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if res = c.StockFacts(ctx, nil, todo, func(FrameRequest, []FrameFact) error {
		t.Fatal("после отмены не качаем")
		return nil
	}); res.Fetched != 0 {
		t.Fatalf("отмена: %+v", res)
	}
	var f *Feed
	for _, x := range NewClient().Feeds() {
		if x.ID == SourceStockFacts {
			x := x
			f = &x
		}
	}
	if !IsSource(SourceStockFacts) || f == nil || f.Facts == nil || f.Prices != nil || f.Unit != "значений" {
		t.Fatalf("фид: %+v", f)
	}
}

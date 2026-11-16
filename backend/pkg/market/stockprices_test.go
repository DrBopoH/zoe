package market

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// stockJSON — месячный ответ Yahoo: метки «полночь 1-го по Нью-Йорку», один месяц без закрытия, последний месяц продублирован.
func stockJSON(adj bool) string {
	ny := time.FixedZone("EST", -5*3600)
	ts := []string{}
	cl := []string{}
	ad := []string{}
	add := func(y int, m time.Month, c, a string) {
		ts = append(ts, fmt.Sprint(time.Date(y, m, 1, 9, 30, 0, 0, ny).Unix()))
		cl, ad = append(cl, c), append(ad, a)
	}
	add(2026, 8, "100", "98")
	add(2026, 9, "null", "null")
	add(2026, 10, "110.5", "108")
	add(2026, 11, "120", "118")
	ts = append(ts, fmt.Sprint(time.Date(2026, 11, 14, 16, 0, 0, 0, ny).Unix())) // дубль текущего месяца: Yahoo присылает свежий отсчёт
	cl, ad = append(cl, "125"), append(ad, "123")
	s := `{"chart":{"result":[{"timestamp":[` + strings.Join(ts, ",") + `],"indicators":{"quote":[{"close":[` + strings.Join(cl, ",") + `]}]`
	if adj {
		s += `,"adjclose":[{"adjclose":[` + strings.Join(ad, ",") + `]}]`
	}
	return s + `}}],"error":null}}`
}

func TestParseStockHistory(t *testing.T) {
	bars, err := parseStockHistory([]byte(stockJSON(true)), "AAPL")
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 3 {
		t.Fatalf("месяц без закрытия пропущен, дубль ноября схлопнут: %+v", bars)
	}
	if bars[0].Date != "2026-08-01" || bars[0].Close != 100 || bars[0].Adj != 98 || bars[0].Ticker != "AAPL" {
		t.Fatalf("август: %+v", bars[0])
	}
	if bars[2].Date != "2026-11-01" || bars[2].Close != 125 || bars[2].Adj != 123 {
		t.Fatalf("ноябрь — поздний отсчёт: %+v", bars[2])
	}
	noAdj, err := parseStockHistory([]byte(stockJSON(false)), "AAPL")
	if err != nil || noAdj[0].Adj != 100 {
		t.Fatalf("нет поправленной цены — Adj равна Close: %v %+v", err, noAdj)
	}
}

func TestParseStockHistoryRejectsBadAnswers(t *testing.T) {
	for _, bad := range []string{
		`не json`,
		`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`,
		`{"chart":{"result":[],"error":null}}`,
		`{"chart":{"result":[{"timestamp":[1],"indicators":{"quote":[{"close":[null]}]}}],"error":null}}`,
	} {
		if _, err := parseStockHistory([]byte(bad), "X"); err == nil {
			t.Errorf("ждали ошибку на %q", bad)
		}
	}
}

func stockClient(t *testing.T, missing ...string) (*Client, *[]string) {
	t.Helper()
	var mu sync.Mutex
	asked := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Mozilla/5.0" {
			http.Error(w, "нужен браузерный User-Agent", http.StatusForbidden)
			return
		}
		tk := strings.TrimPrefix(r.URL.Path, "/v8/finance/chart/")
		mu.Lock()
		asked = append(asked, tk+"?"+r.URL.RawQuery)
		mu.Unlock()
		for _, m := range missing {
			if tk == m {
				http.Error(w, `{"chart":{"result":null,"error":{"description":"No data found"}}}`, http.StatusNotFound)
				return
			}
		}
		w.Write([]byte(stockJSON(true)))
	}))
	t.Cleanup(srv.Close)
	c := NewClient()
	c.YahooBase = srv.URL
	c.Retries = 0
	return c, &asked
}

func TestStockPricesSavesEachTickerAndSkipsMissing(t *testing.T) {
	c, asked := stockClient(t, "GONE")
	got := map[string]int{}
	res := c.StockPrices(context.Background(), nil, []string{"AAPL", "GONE", "BRK-B", "MSFT", "NVDA", "TSLA", "AMZN", "META", "GOOG", "AVGO", "ORCL"}, func(b []PriceBar) error {
		got[b[0].Ticker] += len(b)
		return nil
	})
	if res.Error != "" || res.Source != SourceStockPrices || res.Skipped != 1 || res.Fetched != 30 || len(got) != 10 || got["GONE"] != 0 {
		t.Fatalf("один тикер без истории из 11 — не ошибка источника: %+v %v", res, got)
	}
	if !strings.Contains(strings.Join(*asked, " "), "BRK-B?interval=1mo&range=max") {
		t.Fatalf("класс акций уходит как есть, месячный интервал: %v", *asked)
	}
}

func TestStockPricesReportsWhenFifthIsMissing(t *testing.T) {
	c, _ := stockClient(t, "A", "B")
	res := c.StockPrices(context.Background(), nil, []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}, func([]PriceBar) error { return nil })
	if res.Skipped != 2 || !strings.HasPrefix(res.Error, "без истории 2 из 10: ") {
		t.Fatalf("пятая часть без истории — ошибка источника: %+v", res)
	}
}

func TestStockPricesStopsOnCancelAndSaveFailure(t *testing.T) {
	c, asked := stockClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	res := c.StockPrices(ctx, nil, []string{"A", "B", "C"}, func([]PriceBar) error {
		if n++; n == 1 {
			cancel()
		}
		return nil
	})
	if res.Fetched != 3 || len(*asked) != 1 || res.Error != "" {
		t.Fatalf("после отмены дальше не идём, скачанное остаётся: %+v %v", res, *asked)
	}
	c2, _ := stockClient(t)
	res = c2.StockPrices(context.Background(), nil, []string{"A", "B"}, func([]PriceBar) error { return fmt.Errorf("диск") })
	if res.Skipped != 2 || !strings.Contains(res.Error, "диск") {
		t.Fatalf("сбой сохранения виден: %+v", res)
	}
	c3 := NewClient()
	c3.YahooBase = ""
	if res = c3.StockPrices(context.Background(), nil, []string{"A"}, nil); res.Fetched != 0 || res.Error != "" {
		t.Fatalf("пустой адрес отключает опрос: %+v", res)
	}
}

func TestStockPricesFeedIsRegistered(t *testing.T) {
	if !IsSource(SourceStockPrices) {
		t.Fatal("источник stockprices должен быть известен маршрутам")
	}
	for _, f := range NewClient().Feeds() {
		if f.ID == SourceStockPrices {
			if f.Prices == nil || f.Fetch != nil || f.Series != nil || f.Companies != nil {
				t.Fatalf("фид: %+v", f)
			}
			return
		}
	}
	t.Fatal("фида нет")
}

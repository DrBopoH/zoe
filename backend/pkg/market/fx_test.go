package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fxServer(t *testing.T, fail string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("valcode") == fail && strings.HasPrefix(q.Get("start"), "2006") {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		switch {
		case q.Get("valcode") == "usd" && strings.HasPrefix(q.Get("start"), "1996"):
			w.Write([]byte(`[{"exchangedate":"06.01.1996","rate_per_unit":179900,"units":1},{"exchangedate":"03.09.1996","rate_per_unit":1.76,"units":1}]`))
		case q.Get("valcode") == "xau" && strings.HasPrefix(q.Get("start"), "2016"):
			w.Write([]byte(`[{"exchangedate":"12.11.2026","rate":183562.1,"units":1,"rate_per_unit":183562.1}]`))
		case strings.HasPrefix(q.Get("start"), "2016"):
			w.Write([]byte(`[{"exchangedate":"12.11.2026","rate":45.3222,"units":1},{"exchangedate":"11.11.2026","rate_per_unit":0}]`))
		default:
			w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func fxClient(srv *httptest.Server) *Client {
	c := NewClient()
	c.NBUBase = srv.URL
	c.Now = func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) }
	return c
}

func TestFXNBUParsesKarbovanetsAndFallbacks(t *testing.T) {
	pts, res := fxClient(fxServer(t, "")).FXNBU(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	usd := by[SerUSDUAH]
	if len(usd) != 3 || usd[0].Date != "1996-01-06" || usd[0].Value < 1.79 || usd[0].Value > 1.8 || usd[1].Value != 1.76 || usd[2].Value != 45.3222 {
		t.Fatalf("usd: %+v", usd)
	}
	if g := by[SerXAUUAH]; len(g) != 1 || g[0].Value != 183562.1 {
		t.Fatalf("xau: %+v", g)
	}
	if res.Skipped != 20 {
		t.Fatalf("skipped=%d, want 20 (нулевой курс у usd, eur, всех валют курсов стран, серебра, платины и палладия — кроме золота)", res.Skipped)
	}
}

func TestFXNBUFailedWindowDropsOnlyThatSeries(t *testing.T) {
	pts, res := fxClient(fxServer(t, "eur")).FXNBU(context.Background(), nil)
	by := byID(pts)
	if len(by[SerEURUAH]) != 0 || len(by[SerUSDUAH]) == 0 || len(by[SerXAUUAH]) == 0 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "EUR") {
		t.Fatalf("error: %q", res.Error)
	}
}

// НБУ закончил рубль 31.03.2025: ряд продолжается долларом НБУ ÷ RUB=X Yahoo (форма ответа — живой probe 15.11.2026).
func TestFXNBUExtendsRubWithYahooCrossRate(t *testing.T) {
	var yahooHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v8/finance/chart/RUB=X") {
			yahooHits++
			// 1743465600 = 2025-04-01, 1743552000 = 2025-04-02, 1743638400 = 2025-04-03 (без закрытия)
			w.Write([]byte(`{"chart":{"result":[{"timestamp":[1743379200,1743465600,1743552000,1743638400],"indicators":{"quote":[{"close":[84,83.5,82.5,null]}]}}],"error":null}}`))
			return
		}
		q := r.URL.Query()
		switch {
		case q.Get("valcode") == "usd" && strings.HasPrefix(q.Get("start"), "2016"):
			w.Write([]byte(`[{"exchangedate":"31.03.2025","rate_per_unit":41.5},{"exchangedate":"02.04.2025","rate_per_unit":41.25}]`))
		case q.Get("valcode") == "rub" && strings.HasPrefix(q.Get("start"), "2016"):
			w.Write([]byte(`[{"exchangedate":"31.03.2025","rate":49.5,"units":100,"rate_per_unit":0.495}]`))
		default:
			w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(srv.Close)
	c := fxClient(srv)
	c.YahooBase = srv.URL
	pts, res := c.FXNBU(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	rub := byID(pts)[SerRUBUAH]
	if len(rub) != 3 || rub[0].Value != 0.495 || rub[1].Date != "2025-04-01" || rub[2].Date != "2025-04-02" {
		t.Fatalf("rub: %+v", rub)
	}
	// 1 апреля курса доллара НБУ ещё нет — берётся последний известный (31.03); 2 апреля — свой
	if want := 41.5 / 83.5; rub[1].Value < want-1e-9 || rub[1].Value > want+1e-9 {
		t.Fatalf("01.04: %v, want %v", rub[1].Value, want)
	}
	if want := 41.25 / 82.5; rub[2].Value < want-1e-9 || rub[2].Value > want+1e-9 {
		t.Fatalf("02.04: %v, want %v", rub[2].Value, want)
	}
	if yahooHits != 1 {
		t.Fatalf("Yahoo спрошен %d раз, нужен один", yahooHits)
	}
}

func TestFXNBUKeepsRubWhenYahooFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v8/") {
			http.Error(w, "x", http.StatusForbidden)
			return
		}
		q := r.URL.Query()
		if (q.Get("valcode") == "usd" || q.Get("valcode") == "rub") && strings.HasPrefix(q.Get("start"), "2016") {
			w.Write([]byte(`[{"exchangedate":"31.03.2025","rate_per_unit":0.5}]`))
			return
		}
		w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	c := fxClient(srv)
	c.YahooBase = srv.URL
	c.Retries = 0
	pts, res := c.FXNBU(context.Background(), nil)
	if rub := byID(pts)[SerRUBUAH]; len(rub) != 1 {
		t.Fatalf("рубль НБУ должен остаться: %+v", rub)
	}
	if !strings.Contains(res.Error, "Yahoo") {
		t.Fatalf("error: %q", res.Error)
	}
}

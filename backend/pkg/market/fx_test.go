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
	if res.Skipped != 2 {
		t.Fatalf("skipped=%d, want 2 (нулевой курс у usd и eur)", res.Skipped)
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

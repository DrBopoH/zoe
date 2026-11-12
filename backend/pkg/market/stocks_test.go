package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func stocksServer(t *testing.T, failID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == failID {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		// две точки одной недели + пропуск «.» (выходной)
		w.Write([]byte("observation_date," + id + "\n2026-11-05,6800.5\n2026-11-06,.\n2026-11-09,6900\n2026-11-10,6950.25\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestStocksWeeklyLastClose(t *testing.T) {
	pts, res := econClient(stocksServer(t, "")).Stocks(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	for _, id := range []string{SerStockSP500, SerStockNasdaq, SerStockDow} {
		s := by[id]
		if len(s) != 2 || s[0].Value != 6800.5 || s[1].Date != "2026-11-10" || s[1].Value != 6950.25 {
			t.Fatalf("%s: %+v", id, s)
		}
	}
	if res.Skipped != 3 {
		t.Fatalf("skipped=%d, want 3", res.Skipped)
	}
}

func TestStocksFailedIndexDoesNotKillOthers(t *testing.T) {
	pts, res := econClient(stocksServer(t, "DJIA")).Stocks(context.Background(), nil)
	by := byID(pts)
	if len(by[SerStockDow]) != 0 || len(by[SerStockSP500]) == 0 || len(by[SerStockNasdaq]) == 0 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "DJIA") {
		t.Fatalf("error: %q", res.Error)
	}
}

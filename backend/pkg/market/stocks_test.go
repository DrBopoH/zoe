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
		if failID != "" && id == failID {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		if r.URL.Path == "/datasets/s-and-p-500/main/data/data.csv" {
			w.Write([]byte("Date,SP500,Dividend\n1871-01-01,4.44,0.26\n2026-10-01,7000,0\n2026-11-01,0,0\n2026-11-20,7100,0\n"))
			return
		}
		// две точки одной недели + пропуск «.» (выходной)
		w.Write([]byte("observation_date," + id + "\n2026-11-05,6800.5\n2026-11-06,.\n2026-11-09,6900\n2026-11-10,6950.25\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func stocksClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.ShillerBase = srv.URL
	return c
}

func TestStocksWeeklyLastClose(t *testing.T) {
	pts, res := stocksClient(stocksServer(t, "")).Stocks(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	if s := by[SerStockSP500]; len(s) != 4 || s[0].Date != "1871-01-01" || s[1].Date != "2026-10-01" || s[2].Date != "2026-11-05" || s[3].Value != 6950.25 {
		t.Fatalf("sp500 (история Шиллера до окна FRED + недельный ряд, нулевая точка пропущена, позднее значение истории не дублируется): %+v", s)
	}
	for _, id := range []string{SerStockNasdaq, SerStockDow} {
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
	pts, res := stocksClient(stocksServer(t, "DJIA")).Stocks(context.Background(), nil)
	by := byID(pts)
	if len(by[SerStockDow]) != 0 || len(by[SerStockSP500]) == 0 || len(by[SerStockNasdaq]) == 0 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "DJIA") {
		t.Fatalf("error: %q", res.Error)
	}
}

func TestStocksHistoryFailureKeepsFred(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/datasets/") {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		w.Write([]byte("observation_date,X\n2026-11-09,6900\n"))
	}))
	t.Cleanup(srv.Close)
	pts, res := stocksClient(srv).Stocks(context.Background(), nil)
	if s := byID(pts)[SerStockSP500]; len(s) != 1 || s[0].Value != 6900 {
		t.Fatalf("sp500: %+v", s)
	}
	if !strings.Contains(res.Error, "Шиллер") {
		t.Fatalf("error: %q", res.Error)
	}
}

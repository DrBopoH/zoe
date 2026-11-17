package market

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fredServer(t *testing.T, failID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == failID {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		switch id {
		case "FEDFUNDS":
			w.Write([]byte("observation_date,FEDFUNDS\n2026-08-01,3.63\n2026-09-01,3.75\n"))
		case "DFEDTARU":
			w.Write([]byte("observation_date,DFEDTARU\n2026-11-09,4.00\n2026-11-10,4.00\n2026-11-11,4.25\n2026-11-12,4.25\n"))
		case "CPIAUCSL":
			w.Write([]byte("observation_date,CPIAUCSL\n2025-09-01,300.000\n2026-08-01,330.000\n2026-09-01,333.000\n"))
		case "DGS10":
			// пропуск «.» и две точки одной недели: остаётся последняя
			w.Write([]byte("observation_date,DGS10\n2026-11-05,4.80\n2026-11-06,.\n2026-11-09,4.88\n2026-11-10,4.98\n"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func usClient(srv *httptest.Server) *Client {
	c := NewClient()
	c.FREDBase = srv.URL
	c.Now = func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) }
	return c
}

func TestUSFREDBuildsSeries(t *testing.T) {
	pts, res := usClient(fredServer(t, "")).USFRED(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	if len(by[SerFedFunds]) != 2 {
		t.Fatalf("fed_funds: %+v", by[SerFedFunds])
	}
	if tg := by[SerFedTarget]; len(tg) != 2 || tg[0].Value != 4 || tg[1].Date != "2026-11-11" || tg[1].Value != 4.25 {
		t.Fatalf("fed_target (только смены): %+v", tg)
	}
	yoy := by[SerUSCPIYoY]
	if len(yoy) != 1 || yoy[0].Date != "2026-09-01" || math.Abs(yoy[0].Value-11) > 1e-9 {
		t.Fatalf("yoy: %+v", yoy)
	}
	mom := by[SerUSCPIMoM]
	if len(mom) != 1 || mom[0].Date != "2026-09-01" || math.Abs(mom[0].Value-300.0/330) > 1e-9 {
		t.Fatalf("mom: %+v", mom)
	}
	if u := by[SerUST10Y]; len(u) != 2 || u[0].Value != 4.80 || u[1].Date != "2026-11-10" || u[1].Value != 4.98 {
		t.Fatalf("ust_10y: %+v", u)
	}
	if res.Skipped != 1 {
		t.Fatalf("skipped=%d, want 1 (точка «.»)", res.Skipped)
	}
}

func TestUSFREDFailedSeriesDoesNotKillOthers(t *testing.T) {
	pts, res := usClient(fredServer(t, "CPIAUCSL")).USFRED(context.Background(), nil)
	by := byID(pts)
	if len(by[SerUSCPIYoY]) != 0 || len(by[SerFedFunds]) == 0 || len(by[SerUST10Y]) == 0 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "CPIAUCSL") {
		t.Fatalf("error: %q", res.Error)
	}
}

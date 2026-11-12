package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func econServer(t *testing.T, failID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == failID {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		switch id {
		case "UNRATE":
			w.Write([]byte("observation_date,UNRATE\n2026-08-01,4.3\n2026-09-01,4.4\n"))
		case "A191RL1Q225SBEA":
			w.Write([]byte("observation_date,A191RL1Q225SBEA\n2026-01-01,-0.5\n2026-04-01,2.1\n"))
		case "T10Y2Y":
			// пропуск «.» и две точки одной недели: остаётся последняя
			w.Write([]byte("observation_date,T10Y2Y\n2026-11-05,-0.10\n2026-11-06,.\n2026-11-09,0.05\n2026-11-10,0.12\n"))
		case "VIXCLS":
			w.Write([]byte("observation_date,VIXCLS\n2026-11-09,17.5\n2026-11-10,19.0\n"))
		case "UMCSENT":
			w.Write([]byte("observation_date,UMCSENT\n2026-09-01,55.1\n2026-10-01,.\n"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func econClient(srv *httptest.Server) *Client {
	c := NewClient()
	c.FREDBase = srv.URL
	c.Now = func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) }
	return c
}

func TestUSEconBuildsSeries(t *testing.T) {
	pts, res := econClient(econServer(t, "")).USEcon(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	if u := by[SerUSUnrate]; len(u) != 2 || u[1].Value != 4.4 {
		t.Fatalf("unrate: %+v", u)
	}
	if g := by[SerUSGDP]; len(g) != 2 || g[0].Value != -0.5 {
		t.Fatalf("gdp (отрицательный рост сохраняется): %+v", g)
	}
	if s := by[SerUSSpread]; len(s) != 2 || s[0].Value != -0.10 || s[1].Date != "2026-11-10" || s[1].Value != 0.12 {
		t.Fatalf("spread (неделя → последнее значение): %+v", s)
	}
	if v := by[SerUSVIX]; len(v) != 1 || v[0].Value != 19.0 {
		t.Fatalf("vix: %+v", v)
	}
	if m := by[SerUSSentiment]; len(m) != 1 || m[0].Date != "2026-09-01" {
		t.Fatalf("sentiment (точка «.» пропущена): %+v", m)
	}
	if res.Skipped != 2 {
		t.Fatalf("skipped=%d, want 2", res.Skipped)
	}
}

func TestUSEconFailedSeriesDoesNotKillOthers(t *testing.T) {
	pts, res := econClient(econServer(t, "VIXCLS")).USEcon(context.Background(), nil)
	by := byID(pts)
	if len(by[SerUSVIX]) != 0 || len(by[SerUSUnrate]) == 0 || len(by[SerUSGDP]) == 0 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "VIXCLS") {
		t.Fatalf("error: %q", res.Error)
	}
}

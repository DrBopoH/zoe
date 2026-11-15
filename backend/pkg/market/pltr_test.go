package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func plServer(t *testing.T, failSub string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path + "?" + r.URL.RawQuery
		if failSub != "" && strings.Contains(p, failSub) {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		switch {
		case strings.Contains(p, "LRHUTTTTPLM156S"):
			w.Write([]byte("observation_date,LRHUTTTTPLM156S\n2026-07-01,3.1\n2026-08-01,3.2\n"))
		case strings.Contains(p, "LRHUTTTTTRM156S"):
			w.Write([]byte("observation_date,LRHUTTTTTRM156S\n2026-07-01,8.4\n2026-08-01,8.6\n"))
		case strings.Contains(p, "IRLTLT01PLM156N"):
			w.Write([]byte("observation_date,IRLTLT01PLM156N\n2026-08-01,5.6\n2026-09-01,5.7\n"))
		case strings.Contains(p, "WS_CBPOL") && strings.Contains(p, "M.TR"):
			w.Write([]byte(ezCSV + "x,M,2026-08,45,A\nx,M,2026-09,45,A\nx,M,2026-10,40,A\n"))
		case strings.Contains(p, "WS_CBPOL"):
			w.Write([]byte(ezCSV + "x,M,2026-08,4.0,A\nx,M,2026-09,4.0,A\nx,M,2026-10,3.75,A\n"))
		case strings.Contains(p, "prc_hicp_minr") && strings.Contains(p, ".TR"):
			w.Write([]byte(ezCSV + "x,M,2026-09,33.5,A\nx,M,2026-10,31.2,A\n"))
		case strings.Contains(p, "prc_hicp_minr"):
			w.Write([]byte(ezCSV + "x,M,2026-09,2.9,A\nx,M,2026-10,2.8,A\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func plClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.BISBase, c.EurostatBase = srv.URL, srv.URL
	return c
}

func TestPolandBuildsSeries(t *testing.T) {
	pts, res := plClient(plServer(t, "")).Poland(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourcePL || res.Fetched != len(pts) {
		t.Fatalf("%+v", res)
	}
	if p := by[SerPLPolicy]; len(p) != 2 || p[1].Date != "2026-10-01" || p[1].Value != 3.75 {
		t.Fatalf("ставка — только даты смены значения: %+v", p)
	}
	if len(by[SerPLCPI]) != 2 || len(by[SerPLUnrate]) != 2 || len(by[SerPLY10]) != 2 || by[SerPLY10][1].Value != 5.7 {
		t.Fatalf("%v", by)
	}
}

func TestTurkeyBuildsHighValuesAsIs(t *testing.T) {
	pts, res := plClient(plServer(t, "")).Turkey(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourceTR {
		t.Fatalf("%+v", res)
	}
	if c := by[SerTRCPI]; len(c) != 2 || c[1].Value != 31.2 {
		t.Fatalf("высокая инфляция берётся как есть: %+v", c)
	}
	if p := by[SerTRPolicy]; len(p) != 2 || p[1].Value != 40 {
		t.Fatalf("%+v", p)
	}
	if len(by[SerTRUnrate]) != 2 || len(by) != 3 {
		t.Fatalf("у Турции три ряда (доходности 10 лет нет в проверенных источниках): %v", by)
	}
}

// Сбой одного ряда или источника страны не трогает остальные ряды и соседнюю страну.
func TestPolandPartialFailureKeepsRest(t *testing.T) {
	c := plClient(plServer(t, "prc_hicp_minr"))
	pts, res := c.Poland(context.Background(), nil)
	by := byID(pts)
	if !strings.Contains(res.Error, SerPLCPI) || len(by[SerPLCPI]) != 0 || len(by[SerPLPolicy]) == 0 || len(by[SerPLUnrate]) == 0 || len(by[SerPLY10]) == 0 {
		t.Fatalf("упал только pl_cpi: %q %v", res.Error, by)
	}
	c = plClient(plServer(t, "LRHUTTTTPLM156S"))
	pts, res = c.Poland(context.Background(), nil)
	if !strings.Contains(res.Error, "LRHUTTTTPLM156S") || len(byID(pts)[SerPLY10]) == 0 {
		t.Fatalf("упал только FRED-ряд безработицы: %q", res.Error)
	}
}

func TestPLTRSkipDisabledBases(t *testing.T) {
	c := NewClient()
	c.BISBase, c.EurostatBase, c.FREDBase = "", "", ""
	if pts, res := c.Poland(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("PL: %d %q", len(pts), res.Error)
	}
	if pts, res := c.Turkey(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("TR: %d %q", len(pts), res.Error)
	}
}

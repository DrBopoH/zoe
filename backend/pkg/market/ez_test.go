package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSDMXDate(t *testing.T) {
	for in, want := range map[string]string{"2026-11-10": "2026-11-10", "2026-09": "2026-09-01", "2026-Q1": "2026-01-01", "2026-Q3": "2026-07-01", "2026-Q4": "2026-10-01"} {
		if got, ok := sdmxDate(in); !ok || got != want {
			t.Errorf("sdmxDate(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "2026", "2026-Q5", "abc"} {
		if got, ok := sdmxDate(in); ok {
			t.Errorf("sdmxDate(%q) = %q, ожидалось отклонение", in, got)
		}
	}
}

const ezCSV = "KEY,FREQ,TIME_PERIOD,OBS_VALUE,OBS_STATUS\n"

// ezServer отвечает как ЕЦБ и Евростат вместе: путь решает, какие данные отдать; geoOK — какой код еврозоны у Евростата «живой».
func ezServer(t *testing.T, geoOK string, failSub string, failGeo ...string) *httptest.Server {
	t.Helper()
	fg := ""
	if len(failGeo) > 0 {
		fg = failGeo[0]
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if failSub != "" && strings.Contains(p, failSub) {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		switch {
		case strings.Contains(p, "KR.DFR"):
			w.Write([]byte(ezCSV + "x,D,2026-10-01,2.00,A\nx,D,2026-10-02,2.00,A\nx,D,2026-10-03,1.75,A\n"))
		case strings.Contains(p, "KR.MRR_FR"):
			w.Write([]byte(ezCSV + "x,D,2026-10-01,2.15,A\nx,D,2026-10-03,1.90,A\n"))
		case strings.Contains(p, "EST/"):
			w.Write([]byte(ezCSV + "x,B,2026-11-03,1.93,A\nx,B,2026-11-04,1.94,A\nx,B,2026-11-10,1.92,A\n"))
		case strings.Contains(p, "SR_2Y"):
			w.Write([]byte(ezCSV + "x,B,2026-11-09,2.10,A\nx,B,2026-11-10,2.12,A\n"))
		case strings.Contains(p, "SR_10Y"):
			w.Write([]byte(ezCSV + "x,B,2026-11-10,2.80,A\n"))
		case strings.Contains(p, "prc_hicp_minr"):
			w.Write([]byte(ezCSV + "x,M,2026-09,2.4,A\nx,M,2026-10,2.3,A\n"))
		case strings.Contains(p, "irt_lt_mcby_m"):
			v := map[string]string{"DE": "2.60", "FR": "3.10", "IT": "3.70", "ES": "3.20", "NL": "2.75"}
			for geo, val := range v {
				if strings.HasSuffix(p, "."+geo) {
					if geo == fg {
						http.Error(w, "boom", http.StatusBadGateway)
						return
					}
					w.Write([]byte(ezCSV + "x,M,2026-09," + val + ",A\nx,M,2026-10," + val + ",A\n"))
					return
				}
			}
			http.NotFound(w, r)
		case strings.Contains(p, "une_rt_m"):
			if strings.Contains(p, ".EA") && !strings.HasSuffix(p, "."+geoOK) {
				http.Error(w, "no data", http.StatusNotFound)
				return
			}
			w.Write([]byte(ezCSV + "x,M,2026-08,6.3,A\nx,M,2026-09,6.4,A\n"))
		case strings.Contains(p, "namq_10_gdp"):
			if !strings.HasSuffix(p, "."+geoOK) {
				http.Error(w, "no data", http.StatusNotFound)
				return
			}
			w.Write([]byte(ezCSV + "x,Q,2026-Q1,0.1,A\nx,Q,2026-Q2,0.3,A\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func ezClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.ECBBase, c.EurostatBase = srv.URL, srv.URL
	return c
}

func TestEurozoneBuildsSeries(t *testing.T) {
	pts, res := ezClient(ezServer(t, "EA20", "")).Eurozone(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	if p := by[SerEZPolicy]; len(p) != 2 || p[0].Date != "2026-10-01" || p[1].Value != 1.75 {
		t.Fatalf("policy (только смены значения): %+v", p)
	}
	if m := by[SerEZMRO]; len(m) != 2 || m[1].Value != 1.90 {
		t.Fatalf("mro: %+v", m)
	}
	if e := by[SerEZESTR]; len(e) != 2 || e[1].Date != "2026-11-10" {
		t.Fatalf("estr (неделя → последнее): %+v", e)
	}
	if y := by[SerEZY2]; len(y) != 1 || y[0].Value != 2.12 {
		t.Fatalf("y2: %+v", y)
	}
	if y := by[SerEZY10]; len(y) != 1 || y[0].Value != 2.80 {
		t.Fatalf("y10: %+v", y)
	}
	if c := by[SerEZCPICore]; len(c) != 2 || c[0].Date != "2026-09-01" {
		t.Fatalf("core: %+v", c)
	}
	if u := by[SerEZUnrate]; len(u) != 2 || u[1].Value != 6.4 {
		t.Fatalf("unrate (код EA20 после отказа EA21): %+v", u)
	}
	if g := by[SerEZGDP]; len(g) != 2 || g[1].Date != "2026-04-01" || g[1].Value != 0.3 {
		t.Fatalf("gdp (квартал → первый месяц): %+v", g)
	}
	for _, p := range pts {
		if p.Series == "" {
			t.Fatalf("точка без ряда: %+v", p)
		}
	}
}

func TestEurozoneFailureIsolation(t *testing.T) {
	pts, res := ezClient(ezServer(t, "EA21", "SR_10Y")).Eurozone(context.Background(), nil)
	by := byID(pts)
	if len(by[SerEZY10]) != 0 || len(by[SerEZPolicy]) == 0 || len(by[SerEZGDP]) == 0 {
		t.Fatalf("сбой одного ряда не должен трогать остальные: %v", by)
	}
	if !strings.Contains(res.Error, SerEZY10) || strings.Contains(res.Error, SerEZPolicy) {
		t.Fatalf("ошибка должна называть только упавший ряд: %q", res.Error)
	}
}

func TestEurozoneGeoAllMissingReportsSeries(t *testing.T) {
	_, res := ezClient(ezServer(t, "NOPE", "")).Eurozone(context.Background(), nil)
	if !strings.Contains(res.Error, SerEZUnrate) || !strings.Contains(res.Error, SerEZGDP) {
		t.Fatalf("ни один код еврозоны не подошёл: %q", res.Error)
	}
}

func TestEurozoneSkipsDisabledBases(t *testing.T) {
	c := NewClient()
	c.ECBBase, c.EurostatBase = "", ""
	pts, res := c.Eurozone(context.Background(), nil)
	if len(pts) != 0 || res.Error != "" {
		t.Fatalf("без баз опрашивать нечего: %d %q", len(pts), res.Error)
	}
}

func TestEurozoneRegisteredAndProvidesMatch(t *testing.T) {
	c := NewClient()
	if !IsSource(SourceEZ) {
		t.Fatal("ez не зарегистрирован")
	}
	if got := len(c.Catalog()[SourceEZ]); got != 8+len(ezMemberSeries()) || len(ezMemberSeries()) != 18 {
		t.Fatalf("ez даёт %d рядов, ждали 8 + 18 стран", got)
	}
}

func TestCountriesMetricsAreInCatalog(t *testing.T) {
	known := map[string]bool{}
	for _, ids := range NewClient().Catalog() {
		for _, id := range ids {
			known[id] = true
		}
	}
	seen := map[string]bool{}
	for _, c := range Countries() {
		if c.CC == "" || c.Name == "" || len(c.Metrics) == 0 || seen[c.CC] {
			t.Fatalf("плохая запись страны: %+v", c)
		}
		seen[c.CC] = true
		for m, id := range c.Metrics {
			if !known[id] {
				t.Errorf("%s/%s: ряда %q нет в каталоге", c.CC, m, id)
			}
		}
	}
}

func TestEurozoneMembersAndSpreads(t *testing.T) {
	pts, res := ezClient(ezServer(t, "EA21", "")).Eurozone(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	for _, cc := range ezMembers {
		for _, part := range []string{"cpi", "u", "y10"} {
			if len(by[memberID(cc, part)]) == 0 {
				t.Errorf("нет ряда %s", memberID(cc, part))
			}
		}
	}
	// Италия 3,70 − Германия 2,60 = 1,10 (округление до сотых)
	if s := by[spreadID("it")]; len(s) != 2 || s[1].Date != "2026-10-01" || s[1].Value != 1.10 {
		t.Fatalf("спред Италия − Германия: %+v", s)
	}
	if s := by[spreadID("fr")]; len(s) != 2 || s[0].Value != 0.5 {
		t.Fatalf("спред Франция − Германия: %+v", s)
	}
}

func TestEurozoneMemberFailureKeepsOthers(t *testing.T) {
	pts, res := ezClient(ezServer(t, "EA21", "", "IT")).Eurozone(context.Background(), nil)
	by := byID(pts)
	if len(by["it_y10"]) != 0 || len(by[spreadID("it")]) != 0 {
		t.Fatalf("упавшая страна не должна давать ряд и спред: %v", by["it_y10"])
	}
	if len(by["fr_y10"]) == 0 || len(by[spreadID("fr")]) == 0 || len(by["de_y10"]) == 0 {
		t.Fatal("остальные страны должны остаться")
	}
	if !strings.Contains(res.Error, "it_y10") || strings.Contains(res.Error, "fr_y10") {
		t.Fatalf("ошибка называет только упавший ряд: %q", res.Error)
	}
}

func TestMemberSpreadsNeedBund(t *testing.T) {
	by := map[string][]SeriesPoint{"it_y10": {{Series: "it_y10", Date: "2026-10-01", Value: 3.7}}}
	if got := memberSpreads(by); len(got) != 0 {
		t.Fatalf("без Bund спреда нет: %+v", got)
	}
}

package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const boeSample = "DATE,IUDBEDR\n02 Jan 2024,5.25\n03 Jan 2024,5.25\n01 Aug 2024,5.00\n"

func TestParseBoE(t *testing.T) {
	pts, err := parseBoE([]byte(boeSample), "gb_policy")
	if err != nil || len(pts) != 3 || pts[0].Date != "2024-01-02" || pts[2].Value != 5.00 || pts[1].Series != "gb_policy" {
		t.Fatalf("%v %+v", err, pts)
	}
	for _, bad := range []string{"", "<html>blocked</html>", "DATE,X\n"} {
		if _, err := parseBoE([]byte(bad), "x"); err == nil {
			t.Errorf("ждали ошибку для %q", bad)
		}
	}
}

const onsMonthly = `"Title","CPI ANNUAL RATE"
"CDID","D7G7"
"Source dataset ID","MM23"
"1989","5.2"
"1989 JAN","4.9"
"1989 FEB","5.0"
"2024 Q1","3.1"
"2026 SEP-NOV","4.8"
"2026 OCT","bad"
`

func TestParseONS(t *testing.T) {
	m, err := parseONS([]byte(onsMonthly), "gb_cpi", false)
	if err != nil || len(m) != 3 || m[0].Date != "1989-01-01" || m[2].Date != "2026-11-01" || m[2].Value != 4.8 {
		t.Fatalf("месяцы (скользящий период — по последнему месяцу, годы и кварталы пропущены): %v %+v", err, m)
	}
	q, err := parseONS([]byte(onsMonthly), "gb_gdp", true)
	if err != nil || len(q) != 1 || q[0].Date != "2024-01-01" || q[0].Value != 3.1 {
		t.Fatalf("кварталы: %v %+v", err, q)
	}
	if _, err := parseONS([]byte("<html>blocked</html>"), "x", false); err == nil {
		t.Error("ждали ошибку для HTML")
	}
}

// packServer отвечает как Банк Англии, ONS, BIS и Евростат вместе; failSub — часть пути, на которой отдаёт 502.
func packServer(t *testing.T, failSub string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path + "?" + r.URL.RawQuery
		if failSub != "" && strings.Contains(p, failSub) {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		switch {
		case strings.Contains(p, "IRLTLT01CHM156N"):
			w.Write([]byte("observation_date,IRLTLT01CHM156N\n2026-08-01,0.55\n2026-09-01,0.59\n"))
		case strings.Contains(p, "IUDBEDR"):
			w.Write([]byte("DATE,IUDBEDR\n02 Jan 2024,5.25\n03 Jan 2024,5.25\n01 Aug 2024,5.00\n"))
		case strings.Contains(p, "IUDSOIA"):
			w.Write([]byte("DATE,IUDSOIA\n05 Nov 2026,3.98\n06 Nov 2026,3.99\n12 Nov 2026,3.97\n"))
		case strings.Contains(p, "IUDMNPY"):
			w.Write([]byte("DATE,IUDMNPY\n11 Nov 2026,5.20\n"))
		case strings.Contains(p, "d7g7"):
			w.Write([]byte(onsMonthly))
		case strings.Contains(p, "mgsx"):
			w.Write([]byte("\"Title\",\"Unemployment\"\n\"2026 JUN-AUG\",\"5.0\"\n\"2026 JUL-SEP\",\"5.1\"\n"))
		case strings.Contains(p, "ihyq"):
			w.Write([]byte("\"Title\",\"GDP\"\n\"2026 Q1\",\"0.2\"\n\"2026 Q2\",\"0.3\"\n"))
		case strings.Contains(p, "WS_CBPOL"):
			w.Write([]byte(ezCSV + "x,M,2026-08,0.50,A\nx,M,2026-09,0.50,A\nx,M,2026-10,0.25,A\n"))
		case strings.Contains(p, "prc_hicp_minr"):
			w.Write([]byte(ezCSV + "x,M,2026-09,0.4,A\nx,M,2026-10,0.5,A\n"))
		case strings.Contains(p, "une_rt_m"):
			w.Write([]byte(ezCSV + "x,M,2026-08,2.9,A\nx,M,2026-09,3.0,A\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func packClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.BoEBase, c.ONSBase, c.BISBase, c.EurostatBase = srv.URL, srv.URL, srv.URL, srv.URL
	return c
}

func TestUKBuildsSeries(t *testing.T) {
	pts, res := packClient(packServer(t, "")).UK(context.Background(), nil)
	if res.Error != "" || res.Source != SourceGB {
		t.Fatalf("%+v", res)
	}
	by := byID(pts)
	if p := by[SerGBPolicy]; len(p) != 2 || p[0].Date != "2024-01-02" || p[1].Value != 5.00 {
		t.Fatalf("policy (только смены): %+v", p)
	}
	if s := by[SerGBSonia]; len(s) != 2 || s[1].Date != "2026-11-12" {
		t.Fatalf("sonia (неделя → последнее): %+v", s)
	}
	if len(by[SerGBY10]) != 1 || len(by[SerGBCPI]) != 3 || len(by[SerGBUnrate]) != 2 || by[SerGBUnrate][1].Date != "2026-09-01" {
		t.Fatalf("gilt/cpi/unrate: %v", by)
	}
	if g := by[SerGBGDP]; len(g) != 2 || g[1].Date != "2026-04-01" || g[1].Value != 0.3 {
		t.Fatalf("gdp: %+v", g)
	}
}

func TestUKFailureKeepsOthers(t *testing.T) {
	pts, res := packClient(packServer(t, "mgsx")).UK(context.Background(), nil)
	by := byID(pts)
	if len(by[SerGBUnrate]) != 0 || len(by[SerGBPolicy]) == 0 || len(by[SerGBCPI]) == 0 {
		t.Fatalf("сбой одного ряда не трогает остальные: %v", by)
	}
	if !strings.Contains(res.Error, SerGBUnrate) || strings.Contains(res.Error, SerGBPolicy) {
		t.Fatalf("ошибка называет только упавший ряд: %q", res.Error)
	}
}

func TestSwitzerlandBuildsSeries(t *testing.T) {
	pts, res := packClient(packServer(t, "")).Switzerland(context.Background(), nil)
	if res.Error != "" || res.Source != SourceCH {
		t.Fatalf("%+v", res)
	}
	by := byID(pts)
	if p := by[SerCHPolicy]; len(p) != 2 || p[1].Date != "2026-10-01" || p[1].Value != 0.25 {
		t.Fatalf("policy: %+v", p)
	}
	if y := by[SerCHY10]; len(y) != 2 || y[1].Value != 0.59 {
		t.Fatalf("ch_y10 (FRED): %+v", y)
	}
	if len(by[SerCHCPI]) != 2 || len(by[SerCHUnrate]) != 2 {
		t.Fatalf("cpi/unrate: %v", by)
	}
}

func TestPacksSkipDisabledBases(t *testing.T) {
	c := NewClient()
	c.BoEBase, c.ONSBase, c.BISBase, c.EurostatBase, c.FREDBase = "", "", "", "", ""
	if pts, res := c.UK(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("UK: %d %q", len(pts), res.Error)
	}
	if pts, res := c.Switzerland(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("CH: %d %q", len(pts), res.Error)
	}
}

func TestPacksRegisteredWithSources(t *testing.T) {
	if !IsSource(SourceGB) || !IsSource(SourceCH) || !IsSource(SourceJP) || !IsSource(SourceCN) {
		t.Fatal("gb/ch не зарегистрированы")
	}
	cat := NewClient().Catalog()
	if len(cat[SourceGB]) != 6 || len(cat[SourceCH]) != 4 || len(cat[SourceJP]) != 5 || len(cat[SourceCN]) != 2 {
		t.Fatalf("%v", cat)
	}
	for _, c := range Countries() {
		if c.Source == "" || !IsSource(c.Source) {
			t.Errorf("%s: источник %q не зарегистрирован", c.CC, c.Source)
		}
	}
}

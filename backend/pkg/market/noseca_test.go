package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func nsServer(t *testing.T, failSub string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path + "?" + r.URL.RawQuery
		if failSub != "" && strings.Contains(p, failSub) {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		switch {
		case strings.Contains(p, "LRHUTTTT"):
			w.Write([]byte("observation_date,X\n2026-07-01,4.0\n2026-08-01,4.1\n"))
		case strings.Contains(p, "IRLTLT01"):
			w.Write([]byte("observation_date,X\n2026-08-01,3.3\n2026-09-01,3.4\n"))
		case strings.Contains(p, "WS_CBPOL"):
			w.Write([]byte(ezCSV + "x,M,2026-08,4.0,A\nx,M,2026-09,4.0,A\nx,M,2026-10,3.75,A\n"))
		case strings.Contains(p, "WS_LONG_CPI"):
			w.Write([]byte(bisCPIPct))
		case strings.Contains(p, "prc_hicp_minr"):
			w.Write([]byte(ezCSV + "x,M,2026-09,2.9,A\nx,M,2026-10,2.8,A\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func nsClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.BISBase, c.EurostatBase = srv.URL, srv.URL
	return c
}

func TestNorwaySwedenBuildSeries(t *testing.T) {
	c := nsClient(nsServer(t, ""))
	for _, tc := range []struct {
		name string
		get  func(context.Context, Reporter) ([]SeriesPoint, Result)
		src  string
		ids  []string
	}{
		{"NO", c.Norway, SourceNO, noSeries},
		{"SE", c.Sweden, SourceSE, seSeries},
	} {
		pts, res := tc.get(context.Background(), nil)
		by := byID(pts)
		if res.Error != "" || res.Source != tc.src || res.Fetched != len(pts) || len(by) != len(tc.ids) {
			t.Fatalf("%s: %+v %v", tc.name, res, by)
		}
		for _, id := range tc.ids {
			if len(by[id]) == 0 {
				t.Errorf("%s: нет ряда %s", tc.name, id)
			}
		}
		if p := by[tc.ids[0]]; len(p) != 2 || p[1].Value != 3.75 {
			t.Errorf("%s: ставка — только даты смены значения: %+v", tc.name, p)
		}
	}
}

func TestCanadaCPIFromBIS(t *testing.T) {
	pts, res := nsClient(nsServer(t, "")).Canada(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourceCA || len(by) != 4 {
		t.Fatalf("%+v %v", res, by)
	}
	if c := by[SerCACPI]; len(c) != 2 || c[1].Value != 3.0 {
		t.Fatalf("инфляция Канады из BIS: %+v", c)
	}
}

// Сбой одного источника страны не трогает остальные ряды и соседние страны.
func TestNordicPartialFailure(t *testing.T) {
	c := nsClient(nsServer(t, "prc_hicp_minr"))
	pts, res := c.Sweden(context.Background(), nil)
	by := byID(pts)
	if !strings.Contains(res.Error, SerSECPI) || len(by[SerSECPI]) != 0 || len(by[SerSEPolicy]) == 0 || len(by[SerSEY10]) == 0 {
		t.Fatalf("упал только se_cpi: %q %v", res.Error, by)
	}
	if pts, res := c.Canada(context.Background(), nil); res.Error != "" || len(pts) == 0 {
		t.Fatalf("Канада не зависит от Евростата: %+v", res)
	}
}

func TestNOSECASkipDisabledBases(t *testing.T) {
	c := NewClient()
	c.BISBase, c.EurostatBase, c.FREDBase = "", "", ""
	for name, get := range map[string]func(context.Context, Reporter) ([]SeriesPoint, Result){"NO": c.Norway, "SE": c.Sweden, "CA": c.Canada} {
		if pts, res := get(context.Background(), nil); len(pts) != 0 || res.Error != "" {
			t.Fatalf("%s: %d %q", name, len(pts), res.Error)
		}
	}
}

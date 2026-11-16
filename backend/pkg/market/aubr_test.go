package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func aubrServer(t *testing.T, failSub string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path + "?" + r.URL.RawQuery
		if failSub != "" && strings.Contains(p, failSub) {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		switch {
		case strings.Contains(p, "AXJO"):
			w.Write([]byte(yahooJSON(3000)))
		case strings.Contains(p, "BVSP"):
			w.Write([]byte(yahooJSON(1000)))
		case strings.Contains(p, "LRUNTTTT"):
			w.Write([]byte("observation_date,X\n2026-07-01,4.0\n2026-08-01,4.1\n"))
		case strings.Contains(p, "IRLTLT01"):
			w.Write([]byte("observation_date,X\n2026-08-01,4.3\n2026-09-01,4.4\n"))
		case strings.Contains(p, "TASI"):
			w.Write([]byte(yahooJSON(2600)))
		case strings.Contains(p, "DFMGI"):
			w.Write([]byte(yahooJSON(2400)))
		case strings.Contains(p, "FP.CPI.TOTL.ZG"):
			w.Write([]byte(`[{"page":1,"pages":1,"per_page":200,"total":3},[{"date":"2024","value":32.5},{"date":"2023","value":44.6},{"date":"2022","value":null}]]`))
		case strings.Contains(p, "WS_CBPOL"):
			w.Write([]byte(ezCSV + "x,M,2026-08,3.6,A\nx,M,2026-09,3.6,A\nx,M,2026-10,3.85,A\n"))
		case strings.Contains(p, "WS_LONG_CPI"):
			w.Write([]byte(bisCPIPct))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func aubrClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.BISBase, c.YahooBase = srv.URL, srv.URL
	return c
}

func TestAustraliaBrazilBuildSeries(t *testing.T) {
	c := aubrClient(aubrServer(t, ""))
	for _, tc := range []struct {
		name string
		get  func(context.Context, Reporter) ([]SeriesPoint, Result)
		src  string
		ids  []string
	}{
		{"au", c.Australia, SourceAU, auSeries},
		{"br", c.Brazil, SourceBR, brSeries},
	} {
		pts, res := tc.get(context.Background(), nil)
		by := byID(pts)
		if res.Error != "" || res.Source != tc.src || res.Fetched != len(pts) {
			t.Fatalf("%s: %+v", tc.name, res)
		}
		for _, id := range tc.ids {
			if len(by[id]) == 0 {
				t.Errorf("%s: нет ряда %s (есть %d рядов)", tc.name, id, len(by))
			}
		}
		if len(by) != len(tc.ids) {
			t.Errorf("%s: рядов %d, в Provides %d — список и итог расходятся", tc.name, len(by), len(tc.ids))
		}
	}
	if p := byID(mustPts(c.Australia(context.Background(), nil)))[SerWM("au")]; len(p) != 13 || p[12].Value != 150 {
		t.Fatalf("ASX 200: 3000 / 2000 = 150: %+v", p)
	}
}

func mustPts(pts []SeriesPoint, _ Result) []SeriesPoint { return pts }

func TestAustraliaPartsFailIndependently(t *testing.T) {
	_, res := aubrClient(aubrServer(t, "IRLTLT01")).Australia(context.Background(), nil)
	pts, _ := aubrClient(aubrServer(t, "IRLTLT01")).Australia(context.Background(), nil)
	by := byID(pts)
	if len(by[SerAUY10]) != 0 || len(by[SerAUPolicy]) == 0 || len(by[SerAUCPI]) == 0 || len(by[SerWM("au")]) == 0 || res.Error == "" {
		t.Fatalf("сбой доходности не трогает остальное: %q %v", res.Error, by)
	}
	pts, res = aubrClient(aubrServer(t, "BVSP")).Brazil(context.Background(), nil)
	by = byID(pts)
	if len(by[SerWM("br")]) != 0 || len(by[SerBRPolicy]) == 0 || !strings.Contains(res.Error, "^BVSP") {
		t.Fatalf("сбой Yahoo не трогает BIS и FRED: %q %v", res.Error, by)
	}
}

func TestAustraliaBrazilRegistered(t *testing.T) {
	if !IsSource(SourceAU) || !IsSource(SourceBR) {
		t.Fatal("источники Австралии и Бразилии должны быть известны")
	}
	by := map[string]Country{}
	for _, c := range WithSources(Countries(), NewClient().Catalog()) {
		by[c.CC] = c
	}
	if got := strings.Join(by["au"].Sources, ","); got != "au,fx" {
		t.Errorf("au: свой источник и «Курсы» (AUD к гривне, реальный курс): %s", got)
	}
	if got := strings.Join(by["br"].Sources, ","); got != "br,fx" {
		t.Errorf("br: реал у НБУ не торгуется, но реальный курс — из «Курсов»: %s", got)
	}
}

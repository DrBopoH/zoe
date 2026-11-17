package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pppServer(t *testing.T, failPath string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failPath != "" && strings.Contains(r.URL.String(), failPath) {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "big-mac-full-index.csv"):
			w.Write([]byte("date,iso_a3,currency_code,name,local_price,dollar_ex,dollar_price,USD_raw\n" +
				"2026-01-01,UKR,UAH,Ukraine,150,42,3.571,-0.46\n2026-01-01,USA,USD,United States,6.0,1,6.0,0\n2026-01-01,ZZZ,ZZZ,Nowhere,1,1,1,0\n" +
				"2026-07-01,UKR,UAH,Ukraine,160,43,3.72,-0.38\n"))
		case strings.Contains(r.URL.Path, "/v2/country/"):
			// банк отдаёт от новых к старым, включает значения null и страны, которых мы не просили
			w.Write([]byte(`[{"page":1,"total":4},[` +
				`{"countryiso3code":"UKR","date":"2025","value":515.28},` +
				`{"countryiso3code":"UKR","date":"2024","value":null},` +
				`{"countryiso3code":"UKR","date":"2023","value":400.5},` +
				`{"countryiso3code":"USA","date":"2024","value":143.86},` +
				`{"countryiso3code":"EMU","date":"2024","value":100},` +
				`{"countryiso3code":"AAA","date":"2024","value":1}]]`))
		case r.URL.Query().Get("id") == "CPIAUCSL":
			w.Write([]byte("observation_date,CPIAUCSL\n2000-01-01,169.3\n2026-10-01,337.476\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func pppClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.WBBase, c.ShillerBase = srv.URL, srv.URL
	return c
}

func TestPPPBuildsAllSources(t *testing.T) {
	pts, res := pppClient(pppServer(t, "")).PPP(context.Background(), nil)
	if res.Error != "" || res.Source != SourcePPP || res.Fetched != len(pts) {
		t.Fatalf("%+v", res)
	}
	by := byID(pts)
	// Всемирный банк: старые годы раньше новых, null пропущен, чужая страна не попала, дата — середина года
	u := by[pppCPIPrefix+"ua"]
	if len(u) != 2 || u[0].Date != "2023-07-01" || u[1].Value != 515.28 {
		t.Fatalf("ua cpi: %+v", u)
	}
	if len(by[pppCPIPrefix+"us"]) != 1 {
		t.Fatalf("us cpi: %+v", by[pppCPIPrefix+"us"])
	}
	if len(by[pppCPIPrefix+"ez"]) != 0 {
		t.Fatal("у еврозоны нет индекса цен: запрос без EMU")
	}
	if len(by[pppFXPrefix+"ez"]) != 1 {
		t.Fatalf("курс еврозоны есть: %+v", by[pppFXPrefix+"ez"])
	}
	if len(by["ppp_cpi_aaa"]) != 0 {
		t.Fatal("страна вне набора")
	}
	// FRED: индекс цен США помесячно
	if f := by[SerPPPUSCPI]; len(f) != 2 || f[1].Value != 337.476 {
		t.Fatalf("us cpi idx: %+v", f)
	}
	// Big Mac: две даты по Украине, USD_raw в долях → проценты, чужой код пропущен
	if b := by[bmRawPrefix+"ua"]; len(b) != 2 || b[0].Value != -46 || b[1].Value != -38 || b[1].Date != "2026-07-01" {
		t.Fatalf("bm raw: %+v", b)
	}
	if b := by[bmUSDPrefix+"us"]; len(b) != 1 || b[0].Value != 6 {
		t.Fatalf("bm usd: %+v", b)
	}
	if len(by[bmUSDPrefix+"zzz"]) != 0 {
		t.Fatal("страна вне набора")
	}
}

func TestPPPOneFailureKeepsRest(t *testing.T) {
	for _, tc := range []struct{ fail, lost, kept string }{
		{"FP.CPI.TOTL", pppCPIPrefix + "ua", pppFXPrefix + "ua"},
		{"CPIAUCSL", SerPPPUSCPI, pppCPIPrefix + "ua"},
		{"big-mac", bmRawPrefix + "ua", SerPPPUSCPI},
	} {
		pts, res := pppClient(pppServer(t, tc.fail)).PPP(context.Background(), nil)
		by := byID(pts)
		if res.Error == "" || len(by[tc.lost]) != 0 || len(by[tc.kept]) == 0 {
			t.Errorf("%s: упал только %s: %q", tc.fail, tc.lost, res.Error)
		}
	}
}

func TestPPPCatalogCounts(t *testing.T) {
	// 1 индекс США + 20 + 21 + 20 + 21 + 21 рядов Всемирного банка + 20 стран × 2 ряда Big Mac
	if len(pppSeries) != 1+20+21+20+21+21+40 {
		t.Fatalf("%d", len(pppSeries))
	}
}

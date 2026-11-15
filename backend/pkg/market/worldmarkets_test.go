package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func worldServer(t *testing.T, failID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/data.csv") {
			// 2023: есть PE10 и дивиденд; 2024: нули (Шиллер перестал обновлять); 1871: PE10 нет
			w.Write([]byte("Date,SP500,Dividend,Earnings,Consumer Price Index,Long Interest Rate,Real Price,Real Dividend,Real Earnings,PE10\n" +
				"1871-01-01,4.44,0.26,0.4,12.46,5.32,109.05,6.39,9.82,0.0\n" +
				"2023-06-01,4000,60,181,305,3.75,4359,68,181,29.944\n" +
				"2024-01-01,5000,0.0,0.0,0.0,0.0,0.0,0.0,0.0,0.0\n"))
			return
		}
		id := r.URL.Query().Get("id")
		if id == failID {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		switch id {
		case "NCBEILQ027S":
			w.Write([]byte("observation_date,NCBEILQ027S\n2026-01-01,70000000\n2026-04-01,80000000\n2026-07-01,90000000\n"))
		case "GDP":
			w.Write([]byte("observation_date,GDP\n2026-01-01,30000\n2026-04-01,32000\n"))
		default:
			w.Write([]byte("observation_date," + id + "\n2026-08-01,160.5\n2026-09-01,162.25\n"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func worldClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.ShillerBase = srv.URL
	return c
}

func TestWorldBuildsIndicesAndValuation(t *testing.T) {
	pts, res := worldClient(worldServer(t, "")).World(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourceWorld || res.Fetched != len(pts) {
		t.Fatalf("%+v", res)
	}
	for _, cc := range worldIndexCC {
		if s := by[SerWM(cc)]; len(s) != 2 || s[1].Value != 162.25 || s[1].Date != "2026-09-01" {
			t.Errorf("wm_%s: %+v", cc, s)
		}
	}
	// Баффета: 70 000 млрд / 30 000 = 233,33 %; 80 000 / 32 000 = 250 %; на 2026-07 ВВП нет — точки нет
	if b := by[SerValBuffett]; len(b) != 2 || b[0].Value != 233.33 || b[1].Value != 250 {
		t.Fatalf("buffett: %+v", b)
	}
	// Шиллер: CAPE только там, где PE10 > 0; дивидендная доходность 60/4000 = 1,5 %; нулевые строки пропущены
	if c := by[SerValCAPE]; len(c) != 1 || c[0].Date != "2023-06-01" || c[0].Value != 29.94 {
		t.Fatalf("cape: %+v", c)
	}
	if d := by[SerValDivYld]; len(d) != 2 || d[0].Date != "1871-01-01" || d[1].Value != 1.5 {
		t.Fatalf("divy: %+v", d)
	}
}

func TestWorldOneFailureKeepsRest(t *testing.T) {
	pts, res := worldClient(worldServer(t, "SPASTT01TRM661N")).World(context.Background(), nil)
	by := byID(pts)
	if !strings.Contains(res.Error, "SPASTT01TRM661N") || len(by[SerWM("tr")]) != 0 || len(by[SerWM("us")]) == 0 || len(by[SerValCAPE]) == 0 {
		t.Fatalf("упал только турецкий индекс: %q", res.Error)
	}
	// без Шиллера остаются индексы, без FRED — Шиллер
	c := worldClient(worldServer(t, ""))
	c.ShillerBase = ""
	if pts, _ := c.World(context.Background(), nil); len(byID(pts)[SerValCAPE]) != 0 || len(byID(pts)[SerWM("jp")]) == 0 {
		t.Fatal("без Шиллера нет CAPE, но индексы есть")
	}
	c = worldClient(worldServer(t, ""))
	c.FREDBase = ""
	if pts, _ := c.World(context.Background(), nil); len(byID(pts)[SerWM("jp")]) != 0 || len(byID(pts)[SerValCAPE]) == 0 {
		t.Fatal("без FRED нет индексов, но CAPE есть")
	}
}

func TestBuffettNeedsMatchingDates(t *testing.T) {
	if got := buffett([]fredObs{{Date: "2026-07-01", Value: 1}}, []fredObs{{Date: "2026-04-01", Value: 1}}); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

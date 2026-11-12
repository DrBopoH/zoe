package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Фикстуры повторяют форму живых ответов НБУ (сняты curl'ом 12.11.2026), значения вымышленные.
const keyFixture = `[
 {"dt":"20260101","txt":"Облікова ставка","id_api":"KEY_PolicyRate","freq":"D","tzep":"T90","value":15.5},
 {"dt":"20260102","txt":"Облікова ставка","id_api":"KEY_PolicyRate","freq":"D","tzep":"T90","value":15.5},
 {"dt":"20260103","txt":"Облікова ставка","id_api":"KEY_PolicyRate","freq":"D","tzep":"T90","value":15.5},
 {"dt":"20260104","txt":"Облікова ставка","id_api":"KEY_PolicyRate","freq":"D","tzep":"T90","value":15},
 {"dt":"20260101","txt":"Овернайт сертифікати","id_api":"KEY_RateOvernightCD","freq":"D","value":14.5},
 {"dt":"20260102","txt":"Овернайт сертифікати","id_api":"KEY_RateOvernightCD","freq":"D","value":14.5},
 {"dt":"20260101","txt":"Овернайт кредити","id_api":"KEY_RateOvernightLoans","freq":"D","value":16.5},
 {"dt":"20260101","txt":"Інше","id_api":"KEY_Unknown","freq":"D","value":1}
]`

// общий ИПЦ, базовый, цены производителей, регион, категория — берутся только первые два
const cpiFixture = `[
 {"dt":"20260901","txt":"Індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"Total","mcrk110":"NULL","tzep":"DTPY_","value":6.1},
 {"dt":"20260901","txt":"Індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"Total","mcrk110":"NULL","tzep":"PCPM_","value":0},
 {"dt":"20260901","txt":"Індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"Total","mcrk110":"NULL","tzep":"PCCM_","value":8.1},
 {"dt":"20260901","txt":"Базовий індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"Total","mcrk110":"NULL","tzep":"DTPY_","value":5.6},
 {"dt":"20260901","txt":"Базовий індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"Total","mcrk110":"NULL","tzep":"PCPM_","value":0.5},
 {"dt":"20260901","txt":"Індекс цін виробників промислової продукції ","ku":null,"mcrd081":"Total","mcrk110":"B3","tzep":"DTPY_","value":7.3},
 {"dt":"20260901","txt":"Індекс споживчих цін ","ku":"1","mcrd081":"Total","mcrk110":"NULL","tzep":"DTPY_","value":6},
 {"dt":"20260901","txt":"Індекс споживчих цін ","ku":"7","mcrd081":"10","mcrk110":"NULL","tzep":"DTPY_","value":1.2}
]`

func macroServer(t *testing.T, cpi func(start string) string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/NBUStatService/v1/statdirectory/key", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(keyFixture)) })
	mux.HandleFunc("/NBUStatService/v1/statdirectory/inflation", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("period") != "m" || r.URL.Query().Get("end") == "" {
			w.WriteHeader(http.StatusBadRequest) // без окна сервер отказывает
			return
		}
		w.Write([]byte(cpi(r.URL.Query().Get("start"))))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func byID(pts []SeriesPoint) map[string][]SeriesPoint {
	m := map[string][]SeriesPoint{}
	for _, p := range pts {
		m[p.Series] = append(m[p.Series], p)
	}
	return m
}

func TestMacroNBUCompressesRatesAndPicksHeadlineCPI(t *testing.T) {
	srv := macroServer(t, func(start string) string {
		if start == "20200101" { // только последнее окно содержит данные
			return cpiFixture
		}
		return `[]`
	})
	pts, res := testClient(srv.URL, "").MacroNBU(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("ошибка: %s", res.Error)
	}
	m := byID(pts)
	// ставка: 15.5 держится три дня → одна точка, потом 15
	if g := m[SerPolicyRate]; len(g) != 2 || g[0].Date != "2026-01-01" || g[0].Value != 15.5 || g[1].Date != "2026-01-04" || g[1].Value != 15 {
		t.Fatalf("policy_rate: %+v", g)
	}
	if g := m[SerOvernightCD]; len(g) != 1 || g[0].Value != 14.5 {
		t.Fatalf("overnight_cd: %+v", g)
	}
	if g := m[SerOvernightLoans]; len(g) != 1 || g[0].Value != 16.5 {
		t.Fatalf("overnight_loans: %+v", g)
	}
	// инфляция: только общий и базовый итог, дата — первое число месяца
	one := func(id string, want float64) {
		t.Helper()
		if g := m[id]; len(g) != 1 || g[0].Date != "2026-09-01" || g[0].Value != want {
			t.Fatalf("%s: %+v, ждали %v", id, g, want)
		}
	}
	one(SerCPIYoY, 6.1)
	one(SerCPIMoM, 0)
	one(SerCoreYoY, 5.6)
	one(SerCoreMoM, 0.5)
	if len(m) != 7 {
		t.Fatalf("лишние ряды: %d", len(m))
	}
	if res.Fetched != len(pts) {
		t.Fatalf("fetched %d ≠ %d", res.Fetched, len(pts))
	}
}

func TestMacroNBUKeepsRatesWhenInflationFails(t *testing.T) {
	srv := macroServer(t, func(string) string {
		return `{"message":"Запит повертає надто великий dataset"}`
	})
	pts, res := testClient(srv.URL, "").MacroNBU(context.Background(), nil)
	if len(byID(pts)[SerPolicyRate]) != 2 {
		t.Fatalf("ставки должны сохраниться: %+v", pts)
	}
	if !strings.Contains(res.Error, "инфляция") {
		t.Fatalf("ждали ошибку про инфляцию: %q", res.Error)
	}
}

func TestMacroNBUReportsMissingHeadlineCPI(t *testing.T) {
	srv := macroServer(t, func(string) string { return `[]` })
	_, res := testClient(srv.URL, "").MacroNBU(context.Background(), nil)
	if !strings.Contains(res.Error, "общего индекса") {
		t.Fatalf("пустая инфляция должна быть ошибкой: %q", res.Error)
	}
}

func TestMacroFeedIsRegisteredAndKnown(t *testing.T) {
	var f *Feed
	for _, x := range testClient("", "").Feeds() {
		if x.ID == SourceMacro {
			f = &x
		}
	}
	if f == nil || f.Series == nil || f.Fetch != nil || f.Unit == "" || !IsSource(SourceMacro) || IsSource("x") {
		t.Fatalf("фид macro: %+v", f)
	}
}

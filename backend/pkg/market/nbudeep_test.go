package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Формы ответов сняты живым зондом 15.11.2026; значения вымышленные.
const resFixture = `[
 {"dt":"20250901","txt":"Офіційні резервні активи","id_api":"RES_OffReserveAssets","leveli":1,"freq":"M","tzep":"T071USD_7","value":44000.5},
 {"dt":"20260901","txt":"Офіційні резервні активи","id_api":"RES_OffReserveAssets","leveli":1,"freq":"M","tzep":"T071USD_7","value":48670.23},
 {"dt":"20260901","txt":"Резерви в іноземній валюті","id_api":"RES_ForCurrencyA","leveli":2,"freq":"M","tzep":"T071USD_7","value":47000}
]`

const monFixture = `[
 {"dt":"20250901","txt":"Грошовий агрегат M3","id_api":"M3","leveli":1,"freq":"M","k076":"Total","value":3800000},
 {"dt":"20260901","txt":"Грошовий агрегат M3","id_api":"M3","leveli":1,"freq":"M","k076":"Total","value":4177749.7541},
 {"dt":"20260901","txt":"Грошовий агрегат M3","id_api":"M3","leveli":1,"freq":"M","k076":"UAH","value":3000000},
 {"dt":"20260901","txt":"Грошовий агрегат M2","id_api":"M2","leveli":2,"parent":"M3","freq":"M","k076":"Total","value":4000000},
 {"dt":"20260901","txt":"Готівкові кошти в обігу поза депозитними корпораціями (M0)","id_api":"M0x","freq":"M","k076":"Total","value":1000000}
]`

func deepServer(t *testing.T, res, mon string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/NBUStatService/v1/statdirectory/res", func(w http.ResponseWriter, r *http.Request) {
		if res == "" {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		w.Write([]byte(res))
	})
	mux.HandleFunc("/NBUStatService/v1/statdirectory/monetary", func(w http.ResponseWriter, r *http.Request) {
		if mon == "" {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		w.Write([]byte(mon))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestNBUDeepBuildsReservesAndMoney(t *testing.T) {
	srv := deepServer(t, resFixture, monFixture)
	pts, res := testClient(srv.URL, "").nbuDeep(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("%+v", res)
	}
	by := byID(pts)
	if r := by[SerUAReserves]; len(r) != 2 || r[1].Value != 48.67 || r[1].Date != "2026-09-01" {
		t.Fatalf("резервы: млн долларов → млрд, только официальные резервы: %+v", r)
	}
	if m := by[SerUAM3]; len(m) != 2 || m[1].Value != 4177.75 {
		t.Fatalf("M3 берётся из строки итога, а не из разбивки по валюте: %+v", m)
	}
	if m := by[SerUAM0]; len(m) != 1 || m[0].Value != 1000 {
		t.Fatalf("M0 находится по названию «(M0)»: %+v", m)
	}
	if y := by[SerUAM3YoY]; len(y) != 1 || y[0].Date != "2026-09-01" || y[0].Value != 9.94 {
		t.Fatalf("рост M3 за год: (4177,75/3800 − 1) = 9,94 %%: %+v", y)
	}
	if len(by[SerUAM3YoY])+len(by[SerUAM3])+len(by[SerUAM0])+len(by[SerUAReserves]) != len(pts) {
		t.Fatalf("лишние ряды: %v", by)
	}
}

func TestNBUDeepPartialFailure(t *testing.T) {
	srv := deepServer(t, "", monFixture)
	pts, res := testClient(srv.URL, "").nbuDeep(context.Background(), nil)
	if !strings.Contains(res.Error, "резервы") || len(byID(pts)[SerUAM3]) == 0 {
		t.Fatalf("упали только резервы: %q", res.Error)
	}
	srv = deepServer(t, resFixture, "")
	pts, res = testClient(srv.URL, "").nbuDeep(context.Background(), nil)
	if !strings.Contains(res.Error, "денежные агрегаты") || len(byID(pts)[SerUAReserves]) == 0 {
		t.Fatalf("упали только деньги: %q", res.Error)
	}
}

func TestNBUDeepReportsMissingSeries(t *testing.T) {
	srv := deepServer(t, `[{"dt":"20260901","id_api":"OTHER","value":1}]`, `[{"dt":"20260901","id_api":"M2","value":1}]`)
	_, res := testClient(srv.URL, "").nbuDeep(context.Background(), nil)
	if !strings.Contains(res.Error, "резервы") || !strings.Contains(res.Error, SerUAM3) || !strings.Contains(res.Error, SerUAM0) {
		t.Fatalf("тихо пустой ряд хуже явной ошибки: %q", res.Error)
	}
}

func TestMacroAllMergesCoreAndDeep(t *testing.T) {
	mux := http.NewServeMux()
	core := macroServer(t, func(string) string { return cpiFixture })
	deep := deepServer(t, resFixture, monFixture)
	_ = core
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target := core.URL
		if strings.HasSuffix(r.URL.Path, "/res") || strings.HasSuffix(r.URL.Path, "/monetary") {
			target = deep.URL
		}
		resp, err := http.Get(target + r.URL.Path + "?" + r.URL.RawQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		buf := make([]byte, 1<<16)
		n, _ := resp.Body.Read(buf)
		w.Write(buf[:n])
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	pts, res := testClient(srv.URL, "").MacroAll(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Fetched != len(pts) || len(by[SerPolicyRate]) == 0 || len(by[SerUAReserves]) == 0 || len(by[SerUAM3YoY]) == 0 {
		t.Fatalf("%+v %v", res, by)
	}
}

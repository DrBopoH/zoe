package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

// платёжный баланс: те же строки приходят на каждое окно запроса; в долларах берутся только D021USD_7
const bopFixture = `[
 {"dt":"20260101","txt":"A. Рахунок поточних операцій","id_api":"Cur_acc","leveli":1,"freq":"M","tzep":"D020_7","value":-42962},
 {"dt":"20260101","txt":"A. Рахунок поточних операцій","id_api":"Cur_acc","leveli":1,"freq":"M","tzep":"D021USD_7","value":-1023},
 {"dt":"20260101","txt":"Баланс товарів та послуг","id_api":"G_S","leveli":2,"freq":"M","tzep":"D021USD_7","value":-6810},
 {"dt":"20260101","txt":"Баланс товарів та послуг","id_api":"G_S","leveli":2,"freq":"M","tzep":"D021EUR_7","value":-5815},
 {"dt":"20260101","txt":"Прямі інвестиції","id_api":"FDI","leveli":2,"freq":"M","tzep":"D021USD_7","value":-78}
]`

func deepServer(t *testing.T, res, mon string, bop ...string) *httptest.Server {
	bopBody := bopFixture
	if len(bop) > 0 {
		bopBody = bop[0]
	}
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/NBUStatService/v1/statdirectory/res", func(w http.ResponseWriter, r *http.Request) {
		if res == "" {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		w.Write([]byte(res))
	})
	mux.HandleFunc("/NBUStatService/v1/statdirectory/balanceofpayments", func(w http.ResponseWriter, r *http.Request) {
		if bopBody == "" {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		w.Write([]byte(bopBody))
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
	if ca := by[SerUACA]; len(ca) != 1 || ca[0].Value != -1.02 || ca[0].Date != "2026-01-01" {
		t.Fatalf("текущий счёт в долларах, млн → млрд, одна точка несмотря на повтор в окнах: %+v", ca)
	}
	if gs := by[SerUAGS]; len(gs) != 1 || gs[0].Value != -6.81 {
		t.Fatalf("баланс товаров и услуг: %+v", gs)
	}
	if len(by[SerUAM3YoY])+len(by[SerUAM3])+len(by[SerUAM0])+len(by[SerUAReserves])+len(by[SerUACA])+len(by[SerUAGS]) != len(pts) {
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

func TestNBUDeepBoPFailureKeepsRest(t *testing.T) {
	srv := deepServer(t, resFixture, monFixture, "")
	pts, res := testClient(srv.URL, "").nbuDeep(context.Background(), nil)
	if !strings.Contains(res.Error, "платёжный баланс") || len(byID(pts)[SerUAReserves]) == 0 || len(byID(pts)[SerUAM3]) == 0 {
		t.Fatalf("упал только платёжный баланс: %q", res.Error)
	}
	// пустой ответ без Cur_acc — явная ошибка, а не молчаливо пустой ряд
	srv = deepServer(t, resFixture, monFixture, `[{"dt":"20260101","id_api":"FDI","tzep":"D021USD_7","value":1}]`)
	if _, res = testClient(srv.URL, "").nbuDeep(context.Background(), nil); !strings.Contains(res.Error, "нет текущего счёта") {
		t.Fatalf("%q", res.Error)
	}
}

func TestBoPWindowsCoverLast24Months(t *testing.T) {
	var starts []string
	mux := http.NewServeMux()
	mux.HandleFunc("/NBUStatService/v1/statdirectory/balanceofpayments", func(w http.ResponseWriter, r *http.Request) {
		starts = append(starts, r.URL.Query().Get("start")+"-"+r.URL.Query().Get("end"))
		w.Write([]byte(bopFixture))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	if _, _, err := testClient(srv.URL, "").nbuBoP(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"20241201-20250531", "20250601-20251130", "20251201-20260531", "20260601-20261112"}
	if strings.Join(starts, ",") != strings.Join(want, ",") {
		t.Fatalf("окна: %v, ждали %v (24 месяца по 6, последнее обрезано по сегодня)", starts, want)
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
		if strings.HasSuffix(r.URL.Path, "/res") || strings.HasSuffix(r.URL.Path, "/monetary") || strings.HasSuffix(r.URL.Path, "/balanceofpayments") {
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

// Группы ИПЦ и цены производителей: только национальный уровень (ku пуст) и годовое изменение; регионы, помесячные и прочие отрасли отбрасываются.
func TestInflationGroupsAndPPI(t *testing.T) {
	fixture := `[
 {"dt":"20260901","txt":"Індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"01","mcrk110":"NULL","tzep":"DTPY_","value":7.7},
 {"dt":"20260901","txt":"Індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"01","mcrk110":"NULL","tzep":"PCPM_","value":0.4},
 {"dt":"20260901","txt":"Індекс споживчих цін ","id_api":"prices_price_cpi_","ku":"5","mcrd081":"01","mcrk110":"NULL","tzep":"DTPY_","value":9.9},
 {"dt":"20260901","txt":"Індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"04","mcrk110":"NULL","tzep":"DTPY_","value":12.5},
 {"dt":"20260901","txt":"Індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"99","mcrk110":"NULL","tzep":"DTPY_","value":1},
 {"dt":"20260901","txt":"Індекс цін виробників промислової продукції ","id_api":"prices_price_ppi_","ku":null,"mcrd081":"Total","mcrk110":"BCDE","tzep":"DTPY_","value":5.7},
 {"dt":"20260901","txt":"Індекс цін виробників промислової продукції ","id_api":"prices_price_ppi_","ku":null,"mcrd081":"Total","mcrk110":"B","tzep":"DTPY_","value":3.1},
 {"dt":"20260901","txt":"Індекс споживчих цін ","id_api":"prices_price_cpi_","ku":null,"mcrd081":"Total","mcrk110":"NULL","tzep":"DTPY_","value":6.1}
]`
	srv := macroServer(t, func(start string) string {
		if start == "20200101" {
			return fixture
		}
		return `[]`
	})
	pts, _, _ := testClient(srv.URL, "").inflation(context.Background(), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC))
	by := byID(pts)
	if g := by[SerUACPIGroup("01")]; len(g) != 1 || g[0].Value != 7.7 || g[0].Date != "2026-09-01" {
		t.Fatalf("продукты: только национальный г/г, не регион и не м/м: %+v", g)
	}
	if g := by[SerUACPIGroup("04")]; len(g) != 1 || g[0].Value != 12.5 {
		t.Fatalf("жильё и коммунальные: %+v", g)
	}
	if g := by[SerUAPPI]; len(g) != 1 || g[0].Value != 5.7 {
		t.Fatalf("цены производителей — только вся промышленность (BCDE), не добыча (B): %+v", g)
	}
	if len(by[SerCPIYoY]) != 1 || len(by) != 4 {
		t.Fatalf("общий индекс на месте, лишних рядов нет (код 99 — не группа): %v", by)
	}
}

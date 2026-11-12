package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"economic/pkg/db"
	"economic/pkg/market"
)

func newMarketServer(t *testing.T, usUp bool) *Server {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/depo_securities":
			w.Write([]byte(`[{"cpcode":"UA4000000001","cpdescr":"ОВДП тест","val_code":"UAH","nominal":1000,"auk_proc":16.4,"pgs_date":"2030-02-20"}]`))
		case "/NBU_ovdp":
			w.Write([]byte(`[]`))
		case "/NBUStatService/v1/statdirectory/key":
			w.Write([]byte(`[{"dt":"20260101","id_api":"KEY_PolicyRate","value":15.5},{"dt":"20260102","id_api":"KEY_PolicyRate","value":15.5}]`))
		case "/NBUStatService/v1/statdirectory/inflation":
			if strings.HasPrefix(r.URL.Query().Get("start"), "2020") {
				w.Write([]byte(`[{"dt":"20260901","txt":"Індекс споживчих цін ","ku":null,"mcrd081":"Total","mcrk110":"NULL","tzep":"DTPY_","value":6.1}]`))
			} else {
				w.Write([]byte(`[]`))
			}
		case "/v1/accounting/od/auctions_query":
			if !usUp {
				http.Error(w, "down", http.StatusBadGateway) // US лежит — НБУ всё равно должен сохраниться
				return
			}
			w.Write([]byte(`{"data":[{"cusip":"912810XX1","security_type":"Bond","security_term":"30-Year","auction_date":"2026-11-05","maturity_date":"2056-11-15","int_rate":"4.625"}]}`))
		}
	}))
	t.Cleanup(up.Close)
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := NewServer(st, nil, nil)
	c := market.NewClient()
	c.NBUBase, c.USTBase, c.AuctionDays = up.URL, up.URL, 1
	s.SetMarket(c)
	return s
}

// ждём итог рыночного процесса в /api/sync/status → "market"
func waitMarketLast(t *testing.T, s *Server) map[string]any {
	t.Helper()
	for i := 0; i < 200; i++ {
		_, body := call(t, s.HandleGetStatus, http.MethodGet, "/api/sync/status", "")
		var st struct{ Market map[string]any }
		if err := json.Unmarshal(body, &st); err != nil {
			t.Fatal(err)
		}
		if st.Market["last"] != nil {
			return st.Market["last"].(map[string]any)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("итог рыночного процесса не появился")
	return nil
}

func feedByID(last map[string]any, id string) map[string]any {
	for _, f := range last["feeds"].([]any) {
		if m := f.(map[string]any); m["id"] == id {
			return m
		}
	}
	return nil
}

func TestMarketSyncRunsFeedsIndependently(t *testing.T) {
	s := newMarketServer(t, false)
	if code, _ := call(t, s.HandleMarketSync, http.MethodGet, "/api/market/sync", ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("GET на sync: %d", code)
	}
	if code, _ := call(t, s.HandleMarketSync, http.MethodPost, "/api/market/sync?source=xx", ""); code != http.StatusBadRequest {
		t.Fatalf("плохой source: %d", code)
	}
	if code, body := call(t, s.HandleMarketSync, http.MethodPost, "/api/market/sync", ""); code != 200 || !strings.Contains(string(body), "started") {
		t.Fatalf("старт: %d %s", code, body)
	}
	last := waitMarketLast(t, s)
	if nbu := feedByID(last, "nbu"); nbu["state"] != "done" || nbu["fetched"].(float64) != 1 {
		t.Fatalf("nbu: %+v", nbu)
	}
	if us := feedByID(last, "ust"); us["state"] != "error" || !strings.Contains(us["error"].(string), "502") {
		t.Fatalf("ust должен вернуть ошибку, не ронять НБУ: %+v", us)
	}
	if last["finished_at"].(float64) == 0 {
		t.Fatal("нет времени итога")
	}

	code, body := call(t, s.HandleMarketBonds, http.MethodGet, "/api/market/bonds?source=nbu&q=тест", "")
	var list []market.Security
	if code != 200 || json.Unmarshal(body, &list) != nil || len(list) != 1 || list[0].CouponRate != 16.4 {
		t.Fatalf("список: %d %s", code, body)
	}
	if _, body = call(t, s.HandleMarketBonds, http.MethodGet, "/api/market/bonds?q=нет-такой", ""); strings.TrimSpace(string(body)) != "[]" {
		t.Fatalf("пустая выдача должна быть [], а не null: %s", body)
	}
}

func TestMacroSyncOnlyAndEmptyCache(t *testing.T) {
	s := newMarketServer(t, true)
	if code, body := call(t, s.HandleMacro, http.MethodGet, "/api/macro", ""); code != 200 || !strings.Contains(string(body), `"series":{}`) {
		t.Fatalf("пустой кеш: %d %s", code, body)
	}
	if code, _ := call(t, s.HandleMacro, http.MethodPost, "/api/macro", ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST на macro: %d", code)
	}
	if code, _ := call(t, s.HandleMarketSync, http.MethodPost, "/api/market/sync?source=macro", ""); code != 200 {
		t.Fatalf("source=macro: %d", code)
	}
	last := waitMarketLast(t, s)
	if fs := last["feeds"].([]any); len(fs) != 1 || feedByID(last, "macro")["state"] != "done" {
		t.Fatalf("должен идти только macro: %+v", last)
	}
}

func TestMarketSyncWithoutClient(t *testing.T) {
	s := newMarketServer(t, true)
	s.SetMarket(nil)
	if code, _ := call(t, s.HandleMarketSync, http.MethodPost, "/api/market/sync", ""); code != http.StatusServiceUnavailable {
		t.Fatalf("без клиента ждали 503, а %d", code)
	}
}

// «Разведать» запускает рыночный процесс сам, даже если у банков нет ключей; итоги раздельные
func TestFullSyncStartsMarketToo(t *testing.T) {
	s := newMarketServer(t, true)
	if code, _ := call(t, s.HandleFullHistoricalSync, http.MethodPost, "/api/sync/full", ""); code != 200 {
		t.Fatalf("sync/full: %d", code)
	}
	last := waitMarketLast(t, s)
	if len(last["feeds"].([]any)) != 3 || feedByID(last, "ust")["state"] != "done" || feedByID(last, "nbu")["state"] != "done" || feedByID(last, "macro")["state"] != "done" {
		t.Fatalf("все фиды должны отработать: %+v", last)
	}
	if m := feedByID(last, "macro"); m["unit"] != "точек" || m["fetched"].(float64) != 2 {
		t.Fatalf("macro: ставка (1 точка смены) + инфляция (1 точка): %+v", m)
	}
	_, mb := call(t, s.HandleMacro, http.MethodGet, "/api/macro?series=policy_rate,cpi_yoy", "")
	var mac struct {
		Series    map[string][]db.MacroPoint `json:"series"`
		UpdatedAt int64                      `json:"updated_at"`
	}
	if json.Unmarshal(mb, &mac) != nil || len(mac.Series["policy_rate"]) != 1 || mac.Series["policy_rate"][0].Value != 15.5 || mac.Series["cpi_yoy"][0].Date != "2026-09-01" || mac.UpdatedAt == 0 {
		t.Fatalf("/api/macro: %s", mb)
	}
	if feedByID(last, "nbu")["pct"].(float64) != 100 {
		t.Fatalf("готовый фид — 100%%: %+v", last)
	}
	_, body := call(t, s.HandleGetStatus, http.MethodGet, "/api/sync/status", "")
	var st struct {
		Market struct{ Running bool }
	}
	_ = json.Unmarshal(body, &st)
	if st.Market.Running {
		t.Fatal("после итога опрос не должен числиться идущим")
	}
}

func TestMarketSyncConflictWhileRunning(t *testing.T) {
	s := newMarketServer(t, true)
	s.mk.mu.Lock()
	s.mk.running = true
	s.mk.mu.Unlock()
	if code, _ := call(t, s.HandleMarketSync, http.MethodPost, "/api/market/sync", ""); code != http.StatusConflict {
		t.Fatalf("ждали 409, а %d", code)
	}
}

func TestMarketListsPersistAndNormalize(t *testing.T) {
	s := newMarketServer(t, true)
	if _, body := call(t, s.HandleMarketLists, http.MethodGet, "/api/market/lists?name=icu", ""); !strings.Contains(string(body), `"isins":[]`) {
		t.Fatalf("пустой список должен быть [], а не null: %s", body)
	}
	code, body := call(t, s.HandleMarketLists, http.MethodPut, "/api/market/lists?name=icu",
		`{"isins":["ua4000239040"," UA4000239008","UA4000239040","UA4000239115",""],"prices":{"ua4000239040":1011.12,"UA4000239008":-5,"UA9999999999":1},"ytms":{"UA4000239040":16.15,"UA4000239115":15,"UA4000239008":150}}`)
	if code != 200 || !strings.Contains(string(body), `["UA4000239008","UA4000239040","UA4000239115"]`) {
		t.Fatalf("сохранение: %d %s", code, body)
	}
	if !strings.Contains(string(body), `"prices":{"UA4000239040":1011.12}`) || !strings.Contains(string(body), `"ytms":{"UA4000239040":16.15,"UA4000239115":15}`) {
		t.Fatalf("цена и доходность: только положительные, доходность <100, и только у бумаги из списка: %s", body)
	}
	if _, body = call(t, s.HandleMarketLists, http.MethodGet, "/api/market/lists?name=icu", ""); !strings.Contains(string(body), "1011.12") || !strings.Contains(string(body), `"UA4000239115":15`) {
		t.Fatalf("список не сохранился: %s", body)
	}
	if code, _ = call(t, s.HandleMarketLists, http.MethodGet, "/api/market/lists?name=../x", ""); code != http.StatusBadRequest {
		t.Fatalf("кривое имя должно давать 400, а %d", code)
	}
	// «стереть кеш» (данные пользователя) список не трогает
	if err := s.storage.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if _, body = call(t, s.HandleMarketLists, http.MethodGet, "/api/market/lists?name=icu", ""); !strings.Contains(string(body), "UA4000239040") {
		t.Fatalf("после wipe-cache список пропал: %s", body)
	}
}

func TestMarketCurve(t *testing.T) {
	s := newMarketServer(t, true)
	if _, body := call(t, s.HandleMarketCurve, http.MethodGet, "/api/market/curve?name=icu", ""); !strings.Contains(string(body), `"points":[]`) {
		t.Fatalf("пустая кривая должна быть [], а не null: %s", body)
	}
	code, body := call(t, s.HandleMarketCurve, http.MethodPut, "/api/market/curve?name=icu",
		`{"points":[{"years":3,"ytm":17},{"years":1,"ytm":15.3},{"years":0,"ytm":10},{"years":2,"ytm":-1},{"years":99,"ytm":10}]}`)
	if code != 200 || !strings.Contains(string(body), `[{"years":1,"ytm":15.3},{"years":3,"ytm":17}]`) {
		t.Fatalf("кривая: мусор отбрасывается, точки по сроку: %d %s", code, body)
	}
	if err := s.storage.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if _, body = call(t, s.HandleMarketCurve, http.MethodGet, "/api/market/curve?name=icu", ""); !strings.Contains(string(body), `"ytm":17`) {
		t.Fatalf("после wipe-cache кривая пропала: %s", body)
	}
	if code, _ = call(t, s.HandleMarketCurve, http.MethodGet, "/api/market/curve?name=A%20B", ""); code != http.StatusBadRequest {
		t.Fatalf("кривое имя: %d", code)
	}
}

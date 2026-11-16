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
	"economic/pkg/source"
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
		case "/NBUStatService/v1/statdirectory/res":
			w.Write([]byte(`[{"dt":"20260901","id_api":"RES_OffReserveAssets","tzep":"T071USD_7","value":48670.23}]`))
		case "/NBUStatService/v1/statdirectory/balanceofpayments":
			w.Write([]byte(`[{"dt":"20260101","id_api":"Cur_acc","tzep":"D021USD_7","value":-1023},{"dt":"20260101","id_api":"G_S","tzep":"D021USD_7","value":-6810}]`))
		case "/NBUStatService/v1/statdirectory/monetary":
			w.Write([]byte(`[{"dt":"20260901","txt":"Грошовий агрегат M3","id_api":"M3","k076":"Total","value":4177749.7541},{"dt":"20260901","txt":"Готівкові кошти в обігу поза депозитними корпораціями (M0)","id_api":"M0","k076":"Total","value":1000000}]`))
		case "/graph/fredgraph.csv":
			w.Write([]byte("observation_date," + r.URL.Query().Get("id") + "\n2026-10-01,3.88\n"))
		case "/NBU_Exchange/exchange_site":
			// курс в карбованцах до деноминации: 179900 = 1,799 ₴
			if strings.HasPrefix(r.URL.Query().Get("start"), "1996") {
				w.Write([]byte(`[{"exchangedate":"06.01.1996","rate_per_unit":179900,"units":1},{"exchangedate":"12.11.2006","rate_per_unit":5.05,"units":1}]`))
			} else {
				w.Write([]byte(`[{"exchangedate":"12.11.2026","rate_per_unit":45.3222,"units":1}]`))
			}
		case "/eurostat/api/dissemination/sdmx/2.1/data/prc_hicp_minr/M.RCH_A.TOTAL.EA":
			w.Write([]byte("DATAFLOW,freq,TIME_PERIOD,OBS_VALUE\nX,M,2026-10,2.4\n"))
		case "/datasets/s-and-p-500/main/data/data.csv":
			w.Write([]byte("Date,SP500\n1871-01-01,4.44\n"))
		case "/api/v3/klines":
			w.Write([]byte(`[[1793577600000,"100","110","90","105.5","1",1794182399999]]`))
		case "/fng/":
			w.Write([]byte(`{"data":[{"value":"40","timestamp":"1794009600"}]}`))
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
	c.NBUBase, c.USTBase, c.FREDBase, c.BinanceBase, c.FNGBase, c.ShillerBase, c.EurostatBase, c.ECBBase, c.BoEBase, c.ONSBase, c.BISBase, c.WBBase, c.AuctionDays = up.URL, up.URL, up.URL, up.URL, up.URL, up.URL, up.URL, up.URL, up.URL, up.URL, up.URL, up.URL, 1
	c.P2PBase, c.FuturesBase, c.LlamaBase, c.MempoolBase, c.TronBase, c.EthRPCBase, c.GeckoBase, c.YahooBase = up.URL, up.URL, up.URL, up.URL, up.URL, up.URL, up.URL, up.URL
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
	if code, _ := call(t, s.HandleMarketSync, http.MethodPost, "/api/market/sync?source=macro,xx", ""); code != http.StatusBadRequest {
		t.Fatalf("неизвестный источник в списке: %d", code)
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
	if len(last["feeds"].([]any)) != 38 || feedByID(last, "stocks")["state"] != "done" || feedByID(last, "crypto")["state"] != "done" || feedByID(last, "usecon")["state"] != "done" || feedByID(last, "us")["state"] != "done" || feedByID(last, "fx")["state"] != "done" || feedByID(last, "ust")["state"] != "done" || feedByID(last, "nbu")["state"] != "done" || feedByID(last, "macro")["state"] != "done" {
		t.Fatalf("все фиды должны отработать: %+v", last)
	}
	if m := feedByID(last, "macro"); m["unit"] != "точек" || m["fetched"].(float64) != 8 {
		t.Fatalf("macro: ставка (1 точка смены) + инфляция (1 точка) + еврозона (1 точка) + резервы, M0, M3, текущий счёт, баланс товаров и услуг (по точке): %+v", m)
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

func TestMacroCatalog(t *testing.T) {
	s := newMarketServer(t, true)
	_, body := call(t, s.HandleMacroCatalog, http.MethodGet, "/api/macro/catalog", "")
	var r struct {
		Feeds     map[string][]string `json:"feeds"`
		Countries []struct {
			CC      string   `json:"cc"`
			Sources []string `json:"sources"`
		} `json:"countries"`
	}
	if json.Unmarshal(body, &r) != nil || len(r.Feeds["us"]) != 5 || r.Feeds["us"][0] != "fed_funds" || len(r.Feeds["stocks"]) != 3 {
		t.Fatalf("каталог: %s", body)
	}
	for _, c := range r.Countries {
		if c.CC == "tw" && strings.Join(c.Sources, ",") != "tw,fx" {
			t.Fatalf("карточка Тайваня собирается из своего источника и «Курсов»: %v", c.Sources)
		}
	}
	if code, _ := call(t, s.HandleMacroCatalog, http.MethodPost, "/api/macro/catalog", ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", code)
	}
	s.SetMarket(nil)
	if code, body := call(t, s.HandleMacroCatalog, http.MethodGet, "/api/macro/catalog", ""); code != 200 || !strings.Contains(string(body), `"feeds":{}`) {
		t.Fatalf("без клиента: %d %s", code, body)
	}
}

func TestMacroSourcesHealth(t *testing.T) {
	s := newMarketServer(t, true)
	if code, _ := call(t, s.HandleMacroSources, http.MethodPost, "/api/macro/sources", ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", code)
	}
	if err := s.storage.SaveMacroSeries([]market.SeriesPoint{{Series: "fed_funds", Date: "2000-01-01", Value: 5}, {Series: "fed_funds", Date: "2000-02-01", Value: 5}}, 1); err != nil {
		t.Fatal(err)
	}
	code, body := call(t, s.HandleMacroSources, http.MethodGet, "/api/macro/sources", "")
	var r struct {
		Feeds []struct {
			ID     string `json:"id"`
			Cached int    `json:"cached"`
			Stale  int    `json:"stale"`
		} `json:"feeds"`
		Series []struct {
			ID    string `json:"id"`
			Feed  string `json:"feed"`
			Freq  string `json:"freq"`
			Stale bool   `json:"stale"`
		} `json:"series"`
	}
	if code != 200 || json.Unmarshal(body, &r) != nil || len(r.Series) != 1 || r.Series[0].ID != "fed_funds" || r.Series[0].Feed != "us" || r.Series[0].Freq != "месяц" || !r.Series[0].Stale {
		t.Fatalf("ряд: %d %s", code, body)
	}
	found := false
	for _, f := range r.Feeds {
		if f.ID == "us" {
			found = f.Cached == 1 && f.Stale == 1
		}
	}
	if !found {
		t.Fatalf("источник us: %s", body)
	}
	s.SetMarket(nil)
	if code, body := call(t, s.HandleMacroSources, http.MethodGet, "/api/macro/sources", ""); code != 200 || !strings.Contains(string(body), `"feeds":[]`) {
		t.Fatalf("без клиента: %d %s", code, body)
	}
}

func TestMacroTicksEndpoint(t *testing.T) {
	s := newMarketServer(t, true)
	if code, _ := call(t, s.HandleMacroTicks, http.MethodPost, "/api/macro/ticks?series=p2p_ask", ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", code)
	}
	if code, _ := call(t, s.HandleMacroTicks, http.MethodGet, "/api/macro/ticks?series=us_vix", ""); code != http.StatusBadRequest {
		t.Fatalf("обычный ряд — не снимок: %d", code)
	}
	if code, _ := call(t, s.HandleMacroTicks, http.MethodGet, "/api/macro/ticks?series=p2p_ask&since=abc", ""); code != http.StatusBadRequest {
		t.Fatalf("since: %d", code)
	}
	t.Setenv("ECONOMIC_SOURCES_DIR", t.TempDir())
	slot := int64(1_800_000_000 - 1_800_000_000%3600)
	if err := store().AppendTicks(source.SetChainTicks, 3600, slot, snapshotValues([]market.SeriesPoint{{Series: market.SerP2PAsk, Date: "2026-11-16", Value: 41.3}, {Series: "us_vix", Value: 99}})); err != nil {
		t.Fatal(err)
	}
	code, body := call(t, s.HandleMacroTicks, http.MethodGet, "/api/macro/ticks?series=p2p_ask", "")
	var r []struct {
		T int64   `json:"t"`
		V float64 `json:"v"`
	}
	if code != 200 || json.Unmarshal(body, &r) != nil || len(r) != 1 || r[0].T != slot || r[0].V != 41.3 {
		t.Fatalf("часовая точка: %d %s", code, body)
	}
}

func TestMarketScheduleQuietRunIsHiddenFromStatus(t *testing.T) {
	s := newMarketServer(t, true)
	s.mk.mu.Lock()
	s.mk.running, s.mk.quiet = true, true
	s.mk.mu.Unlock()
	snap := s.marketSnapshot()
	if snap["running"] != false {
		t.Fatalf("фоновый опрос не виден в статусе: %+v", snap)
	}
}

func TestEconomyFilesRebuildMacroCache(t *testing.T) {
	s := newMarketServer(t, true)
	t.Setenv("ECONOMIC_SOURCES_DIR", t.TempDir())
	now := time.Now().Unix()
	// опрос источника «us»: ряды уходят и в кеш, и в файл группы «Экономика»
	pts := []market.SeriesPoint{{Series: "fed_funds", Date: "2026-11-10", Value: 4.0}, {Series: "fed_funds", Date: "2026-11-11", Value: 4.1}, {Series: "us_vix", Date: "2026-11-11", Value: 16}}
	if err := s.storage.SaveMacroSeries(pts, now); err != nil {
		t.Fatal(err)
	}
	if err := s.persistSeries("us", pts); err != nil {
		t.Fatal(err)
	}
	files, _ := store().LoadSeries()
	if len(files["fed_funds"]) != 2 || len(files["us_vix"]) != 1 {
		t.Fatalf("файл набора: %+v", files)
	}
	// часовые снимки за два дня: в кеш вернётся последнее значение каждого дня
	h := int64(3600)
	day := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC).Unix()
	for i, v := range []float64{41.0, 41.2, 41.5} {
		if err := store().AppendTicks(source.SetChainTicks, h, day+int64(i)*14*h, map[string]float64{market.SerP2PAsk: v}); err != nil {
			t.Fatal(err)
		}
	}
	// кеш стёрт, кроме одного свежего ряда: он не перетирается файлом
	if err := s.storage.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if err := s.storage.SaveMacroSeries([]market.SeriesPoint{{Series: "us_vix", Date: "2026-11-12", Value: 99}}, now); err != nil {
		t.Fatal(err)
	}
	n, err := s.RestoreEconomy()
	if err != nil || n != 2 { // fed_funds и p2p_ask; us_vix уже есть
		t.Fatalf("восстановлено рядов: %d %v", n, err)
	}
	got, _, err := s.storage.MacroSeries(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got["fed_funds"]) != 2 || len(got["us_vix"]) != 1 || got["us_vix"][0].Value != 99 {
		t.Fatalf("кеш: %+v", got)
	}
	if p := got[market.SerP2PAsk]; len(p) != 2 || p[1].Value != 41.5 {
		t.Fatalf("снимки по дням: %+v", p)
	}
	if n, _ := s.RestoreEconomy(); n != 0 { // повтор ничего не добавляет
		t.Fatalf("повторное восстановление: %d", n)
	}
}

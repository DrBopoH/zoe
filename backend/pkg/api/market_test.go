package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"economic/pkg/db"
	"economic/pkg/market"
)

func TestMarketSyncAndList(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/depo_securities":
			w.Write([]byte(`[{"cpcode":"UA4000000001","cpdescr":"ОВДП тест","val_code":"UAH","nominal":1000,"auk_proc":16.4,"pgs_date":"2030-02-20"}]`))
		case "/NBU_ovdp":
			w.Write([]byte(`[]`))
		case "/v1/accounting/od/auctions_query":
			http.Error(w, "down", http.StatusBadGateway) // US лежит — НБУ всё равно должен сохраниться
		}
	}))
	defer up.Close()

	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)

	if code, _ := call(t, s.HandleMarketSync, http.MethodPost, "/api/market/sync", ""); code != http.StatusServiceUnavailable {
		t.Fatalf("без клиента ждали 503, а %d", code)
	}
	c := market.NewClient()
	c.NBUBase, c.USTBase, c.AuctionDays = up.URL, up.URL, 1
	s.SetMarket(c)

	if code, _ := call(t, s.HandleMarketSync, http.MethodGet, "/api/market/sync", ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("GET на sync: %d", code)
	}
	if code, _ := call(t, s.HandleMarketSync, http.MethodPost, "/api/market/sync?source=xx", ""); code != http.StatusBadRequest {
		t.Fatalf("плохой source: %d", code)
	}
	code, body := call(t, s.HandleMarketSync, http.MethodPost, "/api/market/sync", "")
	if code != 200 {
		t.Fatalf("sync: %d %s", code, body)
	}
	var out struct{ Results []market.Result }
	if err := json.Unmarshal(body, &out); err != nil || len(out.Results) != 2 {
		t.Fatalf("ответ: %v %s", err, body)
	}
	if out.Results[0].Source != "nbu" || out.Results[0].Fetched != 1 || out.Results[0].Error != "" {
		t.Fatalf("nbu: %+v", out.Results[0])
	}
	if out.Results[1].Source != "ust" || !strings.Contains(out.Results[1].Error, "502") {
		t.Fatalf("ust должен вернуть ошибку, не ронять всё: %+v", out.Results[1])
	}

	code, body = call(t, s.HandleMarketBonds, http.MethodGet, "/api/market/bonds?source=nbu&q=тест", "")
	var list []market.Security
	if code != 200 || json.Unmarshal(body, &list) != nil || len(list) != 1 || list[0].CouponRate != 16.4 {
		t.Fatalf("список: %d %s", code, body)
	}
	if _, body = call(t, s.HandleMarketBonds, http.MethodGet, "/api/market/bonds?q=нет-такой", ""); strings.TrimSpace(string(body)) != "[]" {
		t.Fatalf("пустая выдача должна быть [], а не null: %s", body)
	}
}

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
	if len(last["feeds"].([]any)) != 2 || feedByID(last, "ust")["state"] != "done" || feedByID(last, "nbu")["state"] != "done" {
		t.Fatalf("оба фида должны отработать: %+v", last)
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

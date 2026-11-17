package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func reerServer(t *testing.T, failID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if failID != "" && id == failID {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		w.Write([]byte("observation_date," + id + "\n2026-06-01,101.5\n2026-07-01,102.25\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestREERBuildsAllCountries(t *testing.T) {
	pts, res := econClient(reerServer(t, "")).REER(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourceFX || len(by) != 19 || res.Fetched != len(pts) {
		t.Fatalf("%+v %v", res, by)
	}
	if r := by["reer_ez"]; len(r) != 2 || r[1].Value != 102.25 || r[1].Date != "2026-07-01" {
		t.Fatalf("еврозона = BIS-код XM: %+v", r)
	}
	for _, cc := range []string{"us", "gb", "ch", "jp", "cn", "ru", "pl", "tr", "no", "se", "ca", "kr", "tw", "au", "br", "sa", "ae", "in"} {
		if len(by[SerREER(cc)]) != 2 {
			t.Errorf("нет reer_%s", cc)
		}
	}
}

func TestREEROneFailureKeepsRest(t *testing.T) {
	pts, res := econClient(reerServer(t, "RBTRBIS")).REER(context.Background(), nil)
	by := byID(pts)
	if !strings.Contains(res.Error, "RBTRBIS") || len(by["reer_tr"]) != 0 || len(by["reer_us"]) == 0 || len(by) != 18 {
		t.Fatalf("упал только reer_tr: %q %v", res.Error, len(by))
	}
	c := NewClient()
	c.FREDBase = ""
	if pts, res := c.REER(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("без FRED опрашивать нечего: %d %q", len(pts), res.Error)
	}
}

// Курсы НБУ и реальные курсы — одна «Обновить»; сбой FRED не отнимает курсы, сбой НБУ не отнимает реальные курсы.
func TestFXWorldMergesNBUAndREER(t *testing.T) {
	nbu := fxServer(t, "")
	fred := reerServer(t, "")
	c := fxClient(nbu)
	c.FREDBase = fred.URL
	pts, res := c.FXWorld(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Fetched != len(pts) || len(by[SerUSDUAH]) == 0 || len(by["reer_us"]) == 0 {
		t.Fatalf("%+v", res)
	}
	c.FREDBase = nbu.URL // FRED «упал»: в ответе нет CSV
	pts, res = c.FXWorld(context.Background(), nil)
	if res.Error == "" || len(byID(pts)[SerUSDUAH]) == 0 {
		t.Fatalf("курсы НБУ должны остаться: %q", res.Error)
	}
}

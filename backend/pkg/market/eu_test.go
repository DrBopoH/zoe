package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const euHead = "DATAFLOW,LAST UPDATE,freq,unit,coicop18,geo,TIME_PERIOD,OBS_VALUE,OBS_FLAG,CONF_STATUS\n"

func euServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "prc_hicp_minr/M.RCH_A.TOTAL.EA") {
			w.Write([]byte(body))
			return
		}
		http.Error(w, "x", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func euClient(srv *httptest.Server) *Client {
	c := testClient("", "")
	c.EurostatBase = srv.URL
	return c
}

func TestEuroInflationParsesMonthlyRowsOfUnevenLength(t *testing.T) {
	// строки разной длины (хвост флагов), год без месяца и нечисло пропускаются
	body := euHead + "X,d,M,RCH_A,TOTAL,EA,1997-01,2.1,,\nX,d,M,RCH_A,TOTAL,EA,2026-10,4.0,e\nX,d,M,RCH_A,TOTAL,EA,2026,2.3,,\nX,d,M,RCH_A,TOTAL,EA,2026-09,n/a,,\n"
	pts, err := euClient(euServer(t, body)).euroInflation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 || pts[0].Date != "1997-01-01" || pts[0].Value != 2.1 || pts[1].Date != "2026-10-01" || pts[1].Value != 4.0 || pts[1].Series != SerEUCPIYoY {
		t.Fatalf("points: %+v", pts)
	}
}

func TestEuroInflationErrorsAreExplained(t *testing.T) {
	if _, err := euClient(euServer(t, "No data here")).euroInflation(context.Background()); err == nil || !strings.Contains(err.Error(), "еврозоны") || !strings.Contains(err.Error(), "No data here") {
		t.Fatalf("err=%v", err)
	}
	if _, err := euClient(euServer(t, euHead)).euroInflation(context.Background()); err == nil {
		t.Fatal("пустой набор — ошибка")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", http.StatusBadGateway) }))
	t.Cleanup(bad.Close)
	if _, err := euClient(bad).euroInflation(context.Background()); err == nil {
		t.Fatal("HTTP 502 — ошибка")
	}
}

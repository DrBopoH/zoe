package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const euHead = "DATAFLOW,LAST UPDATE,freq,unit,coicop18,geo,TIME_PERIOD,OBS_VALUE,OBS_FLAG,CONF_STATUS\n"

func euServer(t *testing.T, oldFails bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "prc_hicp_minr"):
			w.Write([]byte(euHead + "X,d,M,RCH_A,TOTAL,EA,2025-12,2.0,,\nX,d,M,RCH_A,TOTAL,EA,2026-10,2.4,e,\nX,d,M,RCH_A,TOTAL,EA,2026,2.3,,\nX,d,M,RCH_A,TOTAL,EA,2026-09,n/a,,\n"))
		case strings.Contains(r.URL.Path, "prc_hicp_manr"):
			if oldFails {
				http.Error(w, "x", http.StatusBadGateway)
				return
			}
			w.Write([]byte(euHead + "X,d,M,RCH_A,CP00,EA,1997-01,2.0,,\nX,d,M,RCH_A,CP00,EA,2025-11,2.1,,\nX,d,M,RCH_A,CP00,EA,2025-12,1.9,,\n"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func euClient(srv *httptest.Server) *Client {
	c := testClient("", "")
	c.EurostatBase = srv.URL
	return c
}

func TestEuroInflationStitchesOldHistoryAndNew(t *testing.T) {
	pts, err := euClient(euServer(t, false)).euroInflation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// старая история до первой точки нового набора (2025-12), дальше новые; год без месяца и n/a пропущены
	want := []string{"1997-01-01", "2025-11-01", "2025-12-01", "2026-10-01"}
	if len(pts) != len(want) {
		t.Fatalf("points: %+v", pts)
	}
	for i, d := range want {
		if pts[i].Date != d || pts[i].Series != SerEUCPIYoY {
			t.Fatalf("%d: %+v, want %s", i, pts[i], d)
		}
	}
	if pts[2].Value != 2.0 || pts[3].Value != 2.4 {
		t.Fatalf("новый набор перекрывает старый на стыке: %+v", pts)
	}
}

func TestEuroInflationOldFailureIsHarmlessNewFailureIsError(t *testing.T) {
	pts, err := euClient(euServer(t, true)).euroInflation(context.Background())
	if err != nil || len(pts) != 2 {
		t.Fatalf("pts=%+v err=%v", pts, err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", http.StatusBadGateway) }))
	t.Cleanup(bad.Close)
	if _, err := euClient(bad).euroInflation(context.Background()); err == nil || !strings.Contains(err.Error(), "еврозоны") {
		t.Fatalf("err=%v", err)
	}
}

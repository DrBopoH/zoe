package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const bisCPIPct = "FREQ,REF_AREA,UNIT_MEASURE,TIME_PERIOD,OBS_VALUE\nM,JP,771,2026-08,2.9\nM,JP,771,2026-09,3.0\nM,JP,628,2026-08,110\nM,JP,628,2026-09,111\n"

func TestParseBISCPIPrefersPercent(t *testing.T) {
	pts, err := parseBISCPI([]byte(bisCPIPct), "jp_cpi")
	if err != nil || len(pts) != 2 || pts[1].Value != 3.0 || pts[0].Date != "2026-08-01" || pts[0].Series != "jp_cpi" {
		t.Fatalf("проценты г/г берутся как есть: %v %+v", err, pts)
	}
}

func TestParseBISCPIFallsBackToIndex(t *testing.T) {
	var b strings.Builder
	b.WriteString("FREQ,REF_AREA,UNIT_MEASURE,TIME_PERIOD,OBS_VALUE\n")
	b.WriteString("M,CN,628,2025-09,100\nM,CN,628,2026-09,103\n")
	pts, err := parseBISCPI([]byte(b.String()), "cn_cpi")
	if err != nil || len(pts) != 1 || pts[0].Date != "2026-09-01" || pts[0].Value < 2.99 || pts[0].Value > 3.01 {
		t.Fatalf("индекс → г/г: %v %+v", err, pts)
	}
	for _, bad := range []string{"", "<html>blocked</html>", "FREQ,X\nM,1\n"} {
		if _, err := parseBISCPI([]byte(bad), "x"); err == nil {
			t.Errorf("ждали ошибку для %q", bad)
		}
	}
}

func jpServer(t *testing.T, failSub string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path + "?" + r.URL.RawQuery
		if failSub != "" && strings.Contains(p, failSub) {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		switch {
		case strings.Contains(p, "LRHUTTTTJPM156S"):
			w.Write([]byte("observation_date,LRHUTTTTJPM156S\n2026-07-01,2.6\n2026-08-01,2.5\n"))
		case strings.Contains(p, "IRLTLT01JPM156N"):
			w.Write([]byte("observation_date,IRLTLT01JPM156N\n2026-09-01,3.05\n"))
		case strings.Contains(p, "NIKKEI225"):
			w.Write([]byte("observation_date,NIKKEI225\n2026-11-05,60000\n2026-11-10,.\n2026-11-12,61000\n2026-11-13,61142.35\n"))
		case strings.Contains(p, "WS_CBPOL"):
			w.Write([]byte(ezCSV + "x,M,2026-08,1.0,A\nx,M,2026-09,1.0,A\nx,M,2026-10,1.25,A\n"))
		case strings.Contains(p, "WS_LONG_CPI"):
			w.Write([]byte(bisCPIPct))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func jpClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.BISBase = srv.URL
	return c
}

func TestJapanBuildsSeries(t *testing.T) {
	pts, res := jpClient(jpServer(t, "")).Japan(context.Background(), nil)
	if res.Error != "" || res.Source != SourceJP || res.Fetched != len(pts) {
		t.Fatalf("%+v (%d точек)", res, len(pts))
	}
	by := byID(pts)
	if p := by[SerJPPolicy]; len(p) != 2 || p[1].Value != 1.25 {
		t.Fatalf("policy (только смены): %+v", p)
	}
	if len(by[SerJPCPI]) != 2 || len(by[SerJPUnrate]) != 2 || len(by[SerJPY10]) != 1 {
		t.Fatalf("cpi/u/y10: %v", by)
	}
	if n := by[SerJPNikkei]; len(n) != 2 || n[1].Value != 61142.35 {
		t.Fatalf("nikkei (пропуск «.» отброшен, неделя → последнее): %+v", n)
	}
}

func TestJapanPartsFailIndependently(t *testing.T) {
	pts, res := jpClient(jpServer(t, "WS_LONG_CPI")).Japan(context.Background(), nil)
	by := byID(pts)
	if len(by[SerJPCPI]) != 0 || len(by[SerJPUnrate]) == 0 || len(by[SerJPPolicy]) == 0 {
		t.Fatalf("сбой BIS-ряда не трогает остальное: %v", by)
	}
	if !strings.Contains(res.Error, SerJPCPI) || strings.Contains(res.Error, SerJPPolicy) {
		t.Fatalf("ошибка называет только упавший ряд: %q", res.Error)
	}
	pts, res = jpClient(jpServer(t, "NIKKEI225")).Japan(context.Background(), nil)
	if len(byID(pts)[SerJPPolicy]) == 0 || !strings.Contains(res.Error, "NIKKEI225") {
		t.Fatalf("сбой FRED-ряда не трогает BIS: %+v", res)
	}
}

func TestChinaBuildsSeries(t *testing.T) {
	pts, res := jpClient(jpServer(t, "")).China(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourceCN || len(by[SerCNPolicy]) != 2 || len(by[SerCNCPI]) != 2 {
		t.Fatalf("%+v %v", res, by)
	}
	c := NewClient()
	c.BISBase = ""
	if pts, res := c.China(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("без BIS опрашивать нечего: %d %q", len(pts), res.Error)
	}
}

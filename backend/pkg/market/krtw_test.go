package market

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// yahooJSON — ответ графика с месячными метками «полночь 1-го числа по Сеулу» (для UTC это конец прошлого дня) и закрытием 2000 весь 2015 год.
func yahooJSON(last float64) string {
	kst := time.FixedZone("KST", 9*3600)
	var ts, cl []string
	for m := 1; m <= 12; m++ {
		ts = append(ts, fmt.Sprint(time.Date(2015, time.Month(m), 1, 0, 0, 0, 0, kst).Unix()))
		cl = append(cl, "2000")
	}
	ts = append(ts, fmt.Sprint(time.Date(2026, 8, 1, 0, 0, 0, 0, kst).Unix()), fmt.Sprint(time.Date(2026, 9, 1, 0, 0, 0, 0, kst).Unix()))
	cl = append(cl, "null", fmt.Sprint(last))
	return `{"chart":{"result":[{"timestamp":[` + strings.Join(ts, ",") + `],"indicators":{"quote":[{"close":[` + strings.Join(cl, ",") + `]}]}}],"error":null}}`
}

func krtwServer(t *testing.T, failSub string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path + "?" + r.URL.RawQuery
		if failSub != "" && strings.Contains(p, failSub) {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		switch {
		case strings.Contains(p, "/v8/finance/chart/%5EKS11") || strings.Contains(p, "/v8/finance/chart/^KS11"):
			if r.Header.Get("User-Agent") == "" || strings.HasPrefix(r.Header.Get("User-Agent"), "Go-http") {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			w.Write([]byte(yahooJSON(3000)))
		case strings.Contains(p, "TWII"):
			w.Write([]byte(yahooJSON(2200)))
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

func krtwClient(srv *httptest.Server) *Client {
	c := econClient(srv)
	c.BISBase, c.YahooBase = srv.URL, srv.URL
	return c
}

func TestParseYahooMonthlyRebasesTo2015(t *testing.T) {
	pts, err := parseYahooMonthly([]byte(yahooJSON(3000)), "wm_kr")
	if err != nil || len(pts) != 13 {
		t.Fatalf("12 месяцев 2015 + сентябрь 2026 (пустое закрытие пропущено): %v %d", err, len(pts))
	}
	if pts[0].Date != "2015-01-01" || pts[0].Value != 100 || pts[11].Date != "2015-12-01" {
		t.Fatalf("метка по Сеулу — всё равно 1-е число своего месяца; среднее 2015 = 100: %+v", pts[:2])
	}
	if l := pts[12]; l.Date != "2026-09-01" || l.Value != 150 || l.Series != "wm_kr" {
		t.Fatalf("3000 / 2000 = 150: %+v", l)
	}
}

func TestParseYahooMonthlyRejectsBadAnswers(t *testing.T) {
	for _, bad := range []string{
		"", "<html>blocked</html>", "Too Many Requests",
		`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found"}}}`,
		`{"chart":{"result":[{"timestamp":[],"indicators":{"quote":[{"close":[]}]}}],"error":null}}`,
		// история есть, а 2015 года нет — привести не к чему
		`{"chart":{"result":[{"timestamp":[1788000000],"indicators":{"quote":[{"close":[100]}]}}],"error":null}}`,
	} {
		if _, err := parseYahooMonthly([]byte(bad), "x"); err == nil {
			t.Errorf("ждали ошибку для %q", bad)
		}
	}
}

func TestKoreaBuildsSeries(t *testing.T) {
	pts, res := krtwClient(krtwServer(t, "")).Korea(context.Background(), nil)
	if res.Error != "" || res.Source != SourceKR || res.Fetched != len(pts) {
		t.Fatalf("%+v", res)
	}
	by := byID(pts)
	if p := by[SerKRPolicy]; len(p) != 2 || p[1].Value != 1.25 {
		t.Fatalf("policy (только смены): %+v", p)
	}
	if len(by[SerKRCPI]) != 2 || len(by[SerWM("kr")]) != 13 {
		t.Fatalf("cpi/kospi: %v", by)
	}
}

func TestKoreaPartsFailIndependently(t *testing.T) {
	pts, res := krtwClient(krtwServer(t, "WS_LONG_CPI")).Korea(context.Background(), nil)
	by := byID(pts)
	if len(by[SerKRCPI]) != 0 || len(by[SerKRPolicy]) == 0 || len(by[SerWM("kr")]) == 0 || !strings.Contains(res.Error, SerKRCPI) {
		t.Fatalf("сбой инфляции не трогает ставку и индекс: %q %v", res.Error, by)
	}
	pts, res = krtwClient(krtwServer(t, "KS11")).Korea(context.Background(), nil)
	by = byID(pts)
	if len(by[SerWM("kr")]) != 0 || len(by[SerKRCPI]) == 0 || !strings.Contains(res.Error, "^KS11") {
		t.Fatalf("сбой Yahoo не трогает BIS: %q %v", res.Error, by)
	}
}

func TestTaiwanBuildsIndex(t *testing.T) {
	pts, res := krtwClient(krtwServer(t, "")).Taiwan(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourceTW || len(by) != 1 || len(by[SerWM("tw")]) != 13 || by[SerWM("tw")][12].Value != 110 {
		t.Fatalf("2200 / 2000 = 110: %+v %v", res, by)
	}
	c := NewClient()
	c.YahooBase = ""
	if pts, res := c.Taiwan(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("без Yahoo опрашивать нечего: %d %q", len(pts), res.Error)
	}
}

func TestKoreaTaiwanRegistered(t *testing.T) {
	if !IsSource(SourceKR) || !IsSource(SourceTW) {
		t.Fatal("источники Кореи и Тайваня должны быть известны")
	}
	by := map[string]Country{}
	for _, c := range Countries() {
		by[c.CC] = c
	}
	if by["kr"].Source != SourceKR || by["kr"].Metrics["policy"] != SerKRPolicy || by["tw"].Source != SourceTW || by["tw"].Metrics["reer"] != SerREER("tw") {
		t.Fatalf("карточки стран: %+v %+v", by["kr"], by["tw"])
	}
}

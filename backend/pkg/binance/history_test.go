package binance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTimelineResidual(t *testing.T) {
	flows := []Flow{{T: 2000, Asset: "USDT", Delta: 100}, {T: 1000, Asset: "USDT", Delta: 300}, {T: 3000, Asset: "BTC", Delta: 1}, {T: 3000, Asset: "USDT", Delta: -50}}
	pts, resid := Timeline(map[string]float64{"USDT": 360, "BTC": 1}, flows, 9000)
	// 300 → 400 → 350 (+ BTC 1); итог движений 350, остаток 360: необъяснённые 10 USDT
	if resid["USDT"] != 10 || len(resid) != 1 {
		t.Fatalf("resid: %v", resid)
	}
	if len(pts) != 4 || pts[0].H["USDT"] != 300 || pts[1].H["USDT"] != 400 || pts[2].H["USDT"] != 350 || pts[2].H["BTC"] != 1 {
		t.Fatalf("points: %+v", pts)
	}
	if last := pts[3]; last.T != 9000 || last.H["USDT"] != 360 {
		t.Fatalf("финальная точка: %+v", last)
	}
}

func histServer(t *testing.T, depositAt, tradeAt, btcJump int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		in := func(ts int64, a, b string) bool {
			x, _ := strconv.ParseInt(q.Get(a), 10, 64)
			y, _ := strconv.ParseInt(q.Get(b), 10, 64)
			return ts >= x && ts <= y
		}
		switch r.URL.Path {
		case "/api/v3/time":
			fmt.Fprint(w, `{"serverTime":`+strconv.FormatInt(time.Now().UnixMilli(), 10)+`}`)
		case "/api/v3/account":
			fmt.Fprint(w, `{"balances":[{"asset":"USDT","free":"400","locked":"0"},{"asset":"BTC","free":"0.01","locked":"0"}]}`)
		case "/sapi/v1/asset/get-funding-asset":
			fmt.Fprint(w, `[]`)
		case "/sapi/v1/simple-earn/flexible/position", "/sapi/v1/simple-earn/locked/position":
			fmt.Fprint(w, `{"rows":[],"total":0}`)
		case "/sapi/v1/capital/deposit/hisrec":
			if in(depositAt, "startTime", "endTime") {
				fmt.Fprintf(w, `[{"amount":"1000","coin":"USDT","status":1,"insertTime":%d}]`, depositAt)
			} else {
				fmt.Fprint(w, `[]`)
			}
		case "/sapi/v1/capital/withdraw/history":
			fmt.Fprint(w, `[]`)
		case "/sapi/v1/convert/tradeFlow":
			fmt.Fprint(w, `{"list":[]}`)
		case "/sapi/v1/fiat/payments", "/sapi/v1/c2c/orderMatch/listUserOrderHistory":
			fmt.Fprint(w, `{"data":[]}`)
		case "/sapi/v1/asset/assetDividend":
			fmt.Fprint(w, `{"rows":[]}`)
		case "/api/v3/exchangeInfo":
			fmt.Fprint(w, `{"symbols":[{"symbol":"BTCUSDT","baseAsset":"BTC","quoteAsset":"USDT"},{"symbol":"USDTUAH","baseAsset":"USDT","quoteAsset":"UAH"},{"symbol":"ETHUSDT","baseAsset":"ETH","quoteAsset":"USDT"},{"symbol":"ABCUSDT","baseAsset":"ABC","quoteAsset":"USDT"}]}`)
		case "/api/v3/myTrades":
			if q.Get("symbol") == "BTCUSDT" && q.Get("fromId") == "0" {
				fmt.Fprintf(w, `[{"id":7,"qty":"0.01","quoteQty":"600","commission":"0","commissionAsset":"BTC","time":%d,"isBuyer":true}]`, tradeAt)
			} else {
				fmt.Fprint(w, `[]`)
			}
		case "/api/v3/klines":
			from, _ := strconv.ParseInt(q.Get("startTime"), 10, 64)
			to, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
			price := func(ts int64) string {
				switch q.Get("symbol") {
				case "USDTUAH":
					return "40"
				case "BTCUSDT":
					if ts >= btcJump {
						return "70000"
					}
					return "60000"
				}
				return "1"
			}
			var rows []string
			for ts := from / 60000 * 60000; ts <= to && len(rows) < 1000; ts += 60000 {
				if ts < from {
					continue
				}
				rows = append(rows, fmt.Sprintf(`[%d,"0","0","0","%s","0",0,"0",0,"0","0","0"]`, ts, price(ts)))
			}
			fmt.Fprint(w, "["+strings.Join(rows, ",")+"]")
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestBuildHistory(t *testing.T) {
	utc := func(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, time.UTC) }
	dep, trd, jump := utc(8, 10, 0), utc(9, 10, 0), utc(9, 12, 0)
	srv := histServer(t, dep.UnixMilli(), trd.UnixMilli(), jump.UnixMilli())
	defer srv.Close()
	c := NewClient("K", "S")
	c.SetBaseURL(srv.URL)
	c.Sleep = func(time.Duration) {}

	now := utc(10, 12, 0)
	h, err := c.BuildHistory(HistoryOpts{From: utc(1, 0, 0), Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Residual) != 0 {
		t.Fatalf("необъяснённое: %v warn=%v", h.Residual, h.Warnings)
	}
	s := h.Series
	at := func(tm time.Time) float64 { return s.Values[(tm.UnixMilli()-s.StartMs)/60000] }
	if s.StartMs != dep.UnixMilli() {
		t.Fatalf("ряд начинается с первого движения: %d", s.StartMs)
	}
	for _, c := range []struct {
		when time.Time
		want float64
	}{
		{utc(8, 10, 30), 40000},                 // 1000 USDT × 40
		{utc(9, 11, 0), 400*40 + 0.01*60000*40}, // после покупки 0.01 BTC по 60000
		{utc(10, 6, 0), 400*40 + 0.01*70000*40}, // BTC вырос — оценка выросла без единой операции
		{now, 400*40 + 0.01*70000*40},
	} {
		if got := at(c.when); got != c.want {
			t.Errorf("%v: %v, ждали %v", c.when, got, c.want)
		}
	}

	// дописывание живого ряда: остатки те же, цены те же
	cur := Holdings{"USDT": {Spot: 400}, "BTC": {Spot: 0.01}}
	e, err := c.ExtendSeries(cur, now.UnixMilli(), now.Add(10*time.Minute).UnixMilli())
	if err != nil || len(e.Values) != 11 || e.Values[10] != 44000 {
		t.Fatalf("extend: %v %+v", err, e)
	}
}

func TestRateLimitRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, `{"serverTime":1}`)
	}))
	defer srv.Close()
	c := NewClient("K", "S")
	c.SetBaseURL(srv.URL)
	var slept time.Duration
	c.Sleep = func(d time.Duration) { slept += d }
	if err := c.SyncTime(); err != nil || calls != 3 || slept != 4*time.Second {
		t.Fatalf("err=%v calls=%d slept=%v", err, calls, slept)
	}
}

// Продажа монеты, покупку которой API не показал (Alpha, стейкинг, раздача): в учёте остаётся «дыра» −N монет.
// Раньше отрицательный остаток отбрасывался, USDT от продажи оставались на графике, а в конце всё схлопывалось обрывом.
func TestSeriesNegativeHoleOffsetsProceeds(t *testing.T) {
	utc := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, time.UTC) }
	srv := histServer(t, utc(8, 10).UnixMilli(), utc(9, 10).UnixMilli(), utc(30, 0).UnixMilli())
	defer srv.Close()
	c := NewClient("K", "S")
	c.SetBaseURL(srv.URL)
	c.Sleep = func(time.Duration) {}
	syms, err := c.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	// 1000 USDT; потом «продали» 100 ABC (цена 1 USDT) за 100 USDT, купленные вне API; в конце на счёте 900 USDT
	flows := []Flow{{T: utc(8, 10).UnixMilli(), Asset: "USDT", Delta: 1000}, {T: utc(9, 10).UnixMilli(), Asset: "ABC", Delta: -100}, {T: utc(9, 10).UnixMilli(), Asset: "USDT", Delta: 100}}
	end := utc(10, 12)
	pts, resid := Timeline(map[string]float64{"USDT": 1000}, flows, end.UnixMilli())
	if resid["USDT"] != -100 || resid["ABC"] != 100 {
		t.Fatalf("resid: %v", resid)
	}
	s, err := c.Series(pts, utc(8, 10).UnixMilli(), end.UnixMilli(), syms, nil)
	if err != nil {
		t.Fatal(err)
	}
	at := func(tm time.Time) float64 { return s.Values[(tm.UnixMilli()-s.StartMs)/60000] }
	// до продажи 1000 USDT; после — 1100 USDT и дыра −100 ABC: те же 1000; в конце снова 1000 — обрыва нет
	for _, tm := range []time.Time{utc(8, 12), utc(9, 12), utc(10, 11), end} {
		if got := at(tm); got != 40000 {
			t.Errorf("%v: %v, ждали 40000", tm, got)
		}
	}
}

// Вынужденная пауза (лимит) сообщается наружу: «остывание» начинается и заканчивается; короткие паузы — нет.
func TestWaitHookReportsCooling(t *testing.T) {
	c := NewClient("K", "S")
	c.Sleep = func(time.Duration) {}
	var got []time.Duration
	c.hk = histHooks{wait: func(d time.Duration) { got = append(got, d) }}
	c.sleep(120 * time.Millisecond)
	c.sleep(5 * time.Second)
	if len(got) != 2 || got[0] != 5*time.Second || got[1] != 0 {
		t.Fatalf("паузы: %v", got)
	}
}

// Окна обходятся от сегодня к прошлому (как у банков); сетка привязана к началу, поэтому границы окон не зависят от «сейчас».
func TestWindowsWalkNewestFirst(t *testing.T) {
	c := NewClient("K", "S")
	var starts, ends []int64
	c.hk = histHooks{start: func(_ string, a, b int64) { starts = append(starts, a); ends = append(ends, b) }}
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(200 * 24 * time.Hour)
	var out []Flow
	if err := c.eachWindow("", from, to, 89*day, &out, func(a, b time.Time) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(starts) != 3 || !(starts[0] > starts[1] && starts[1] > starts[2]) {
		t.Fatalf("окна должны идти от новых к старым: %v", starts)
	}
	if starts[2] != from.UnixMilli() || ends[0] != to.UnixMilli() || ends[1] != starts[0] {
		t.Fatalf("сетка от начала, правый край — сегодня: %v %v", starts, ends)
	}
}

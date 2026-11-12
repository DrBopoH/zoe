package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"economic/pkg/binance"
	"economic/pkg/db"
)

func TestHTTPBinanceSync(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/time":
			fmt.Fprint(w, `{"serverTime":1700000000000}`)
		case r.URL.Path == "/api/v3/ticker/price":
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","price":"60000"},{"symbol":"USDTUAH","price":"41"}]`)
		case r.URL.Path == "/api/v3/account":
			fmt.Fprint(w, `{"balances":[{"asset":"BTC","free":"0.01","locked":"0"}]}`)
		case strings.HasPrefix(r.URL.Path, "/sapi/"):
			w.WriteHeader(403)
			fmt.Fprint(w, `{"code":-2015,"msg":"no permission"}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer fake.Close()

	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)

	// без ключа: интеграция выключена, синк отказывает понятным текстом
	c, b := call(t, s.HandleBinance, "POST", "/api/binance", "")
	if c != 200 || !strings.Contains(string(b), `"configured":false`) || !strings.Contains(string(b), "не подключён") {
		t.Fatalf("без ключа: %d %s", c, b)
	}

	cl := binance.NewClient("K", "S")
	cl.SetBaseURL(fake.URL)
	s.SetBinance(cl)
	c, b = call(t, s.HandleBinance, "POST", "/api/binance", "")
	var out struct {
		Configured bool   `json:"configured"`
		Days       int    `json:"days"`
		Error      string `json:"error"`
		Snapshot   *struct {
			Total float64 `json:"total"`
		} `json:"snapshot"`
	}
	json.Unmarshal(b, &out)
	// 0.01 BTC × 60000 × 41 = 24600 ₴; Funding/Earn без прав — не ошибка
	if c != 200 || !out.Configured || out.Error != "" || out.Snapshot == nil || out.Snapshot.Total != 24600 || out.Days != 1 {
		t.Fatalf("синк: %d %s", c, b)
	}
	// счёт виден в /api/accounts
	c, b = call(t, s.HandleGetAccounts, "GET", "/api/accounts", "")
	if c != 200 || !strings.Contains(string(b), `"bank_key":"binance"`) {
		t.Fatalf("accounts: %d %s", c, b)
	}
}

func newHistFake(dep, trd time.Time) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/api/v3/time":
			fmt.Fprintf(w, `{"serverTime":%d}`, time.Now().UnixMilli())
		case "/api/v3/account":
			fmt.Fprint(w, `{"balances":[{"asset":"USDT","free":"400","locked":"0"},{"asset":"BTC","free":"0.01","locked":"0"}]}`)
		case "/api/v3/ticker/price":
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","price":"60000"},{"symbol":"USDTUAH","price":"40"}]`)
		case "/sapi/v1/capital/deposit/hisrec":
			x, _ := strconv.ParseInt(q.Get("startTime"), 10, 64)
			y, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
			if dep.UnixMilli() >= x && dep.UnixMilli() <= y {
				fmt.Fprintf(w, `[{"amount":"1000","coin":"USDT","status":1,"insertTime":%d}]`, dep.UnixMilli())
			} else {
				fmt.Fprint(w, `[]`)
			}
		case "/sapi/v1/capital/withdraw/history", "/sapi/v1/asset/get-funding-asset":
			fmt.Fprint(w, `[]`)
		case "/sapi/v1/convert/tradeFlow":
			fmt.Fprint(w, `{"list":[]}`)
		case "/sapi/v1/fiat/payments", "/sapi/v1/c2c/orderMatch/listUserOrderHistory":
			fmt.Fprint(w, `{"data":[]}`)
		case "/sapi/v1/asset/assetDividend", "/sapi/v1/simple-earn/flexible/position", "/sapi/v1/simple-earn/locked/position":
			fmt.Fprint(w, `{"rows":[]}`)
		case "/api/v3/exchangeInfo":
			fmt.Fprint(w, `{"symbols":[{"symbol":"BTCUSDT","baseAsset":"BTC","quoteAsset":"USDT"},{"symbol":"USDTUAH","baseAsset":"USDT","quoteAsset":"UAH"}]}`)
		case "/api/v3/myTrades":
			if q.Get("symbol") == "BTCUSDT" && q.Get("fromId") == "0" {
				fmt.Fprintf(w, `[{"id":1,"qty":"0.01","quoteQty":"600","commission":"0","commissionAsset":"BTC","time":%d,"isBuyer":true}]`, trd.UnixMilli())
			} else {
				fmt.Fprint(w, `[]`)
			}
		case "/api/v3/klines":
			from, _ := strconv.ParseInt(q.Get("startTime"), 10, 64)
			to, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
			p := "1"
			switch q.Get("symbol") {
			case "USDTUAH":
				p = "40"
			case "BTCUSDT":
				p = "60000"
			}
			var rows []string
			for ts := (from + 59999) / 60000 * 60000; ts <= to && len(rows) < 1000; ts += 60000 {
				rows = append(rows, fmt.Sprintf(`[%d,"0","0","0","%s","0",0,"0",0,"0","0","0"]`, ts, p))
			}
			fmt.Fprint(w, "["+strings.Join(rows, ",")+"]")
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestHTTPBinanceHistoryAndCandles(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	dep := now.Add(-3 * time.Hour)
	trd := now.Add(-2 * time.Hour)
	fake := newHistFake(dep, trd)
	defer fake.Close()

	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)
	cl := binance.NewClient("K", "S")
	cl.SetBaseURL(fake.URL)
	cl.Sleep = func(time.Duration) {}
	s.SetBinance(cl)
	s.SetBinanceHistoryFrom(now.Add(-24 * time.Hour))

	if c, b := call(t, s.HandleBinanceHistory, "POST", "/api/binance/history", ""); c != 200 {
		t.Fatalf("старт: %d %s", c, b)
	}
	for i := 0; i < 200; i++ { // ждём конца фоновой задачи
		_, b := call(t, s.HandleBinanceHistory, "GET", "/api/binance/history", "")
		if !strings.Contains(string(b), `"running":true`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	_, b := call(t, s.HandleBinanceHistory, "GET", "/api/binance/history", "")
	var hv struct {
		Status struct {
			Running  bool               `json:"running"`
			Error    string             `json:"error"`
			Residual map[string]float64 `json:"residual"`
		} `json:"status"`
		First  int64 `json:"first"`
		Points int   `json:"points"`
	}
	json.Unmarshal(b, &hv)
	if hv.Status.Running || hv.Status.Error != "" || len(hv.Status.Residual) != 0 || hv.First != dep.Unix() || hv.Points < 180 {
		t.Fatalf("история: %s", b)
	}

	// свечи часовыми корзинами: последняя корзина — цена 400*40 + 0.01*60000*40 = 40000
	hour := now.Truncate(time.Hour)
	body := fmt.Sprintf(`{"starts":[%d,%d,%d],"end":%d}`, hour.Add(-4*time.Hour).Unix(), hour.Add(-time.Hour).Unix(), hour.Unix(), now.Unix()+60)
	c, b := call(t, s.HandleBinanceCandles, "POST", "/api/binance/candles", body)
	var cv struct {
		Has       bool         `json:"has"`
		LastValue float64      `json:"last_value"`
		Candles   [][4]float64 `json:"candles"`
	}
	json.Unmarshal(b, &cv)
	if c != 200 || !cv.Has || cv.LastValue != 40000 || len(cv.Candles) != 3 || cv.Candles[2][3] != 40000 {
		t.Fatalf("свечи: %d %s", c, b)
	}
}

func TestCrawlBinanceInProgressPanel(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	fake := newHistFake(now.Add(-3*time.Hour), now.Add(-2*time.Hour))
	defer fake.Close()
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)
	cl := binance.NewClient("K", "S")
	cl.SetBaseURL(fake.URL)
	cl.Sleep = func(time.Duration) {}
	s.SetBinance(cl)
	s.SetBinanceHistoryFrom(now.Add(-24 * time.Hour))

	s.prog.begin()
	s.crawlBinance() // синхронно: оценка портфеля, затем история
	s.prog.finish()

	v := s.progressSnapshot()
	if len(v.Clients) != 1 || v.Clients[0].Name != "Binance" || v.Clients[0].Kind != "binance" || len(v.Clients[0].Accounts) != 2 {
		t.Fatalf("панель: %+v", v.Clients)
	}
	c := v.Clients[0]
	for _, a := range c.Accounts {
		if a.State != "done" || a.Percent != 100 || strings.Contains(a.Cells, "0") {
			t.Fatalf("счёт %s: %+v", a.Label, a)
		}
	}
	if !c.Finished || c.Percent != 100 {
		t.Fatalf("клиент: %+v", c)
	}
	// второй «Разведать»: история уже есть — показываем только портфель
	s.prog.begin()
	s.crawlBinance()
	s.prog.finish()
	if v := s.progressSnapshot(); len(v.Clients[0].Accounts) != 1 {
		t.Fatalf("повторно: %+v", v.Clients[0].Accounts)
	}
}

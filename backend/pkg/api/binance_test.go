package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"economic/pkg/binance"
	"economic/pkg/db"
	"economic/pkg/mono"
)

func newHistFake(dep, trd time.Time, spy ...func(path string) (fail bool)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, f := range spy {
			if f(r.URL.Path) {
				w.WriteHeader(500)
				return
			}
		}
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
			if !dep.IsZero() && dep.UnixMilli() >= x && dep.UnixMilli() <= y {
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
			if !trd.IsZero() && q.Get("symbol") == "BTCUSDT" && q.Get("fromId") == "0" {
				fmt.Fprintf(w, `[{"id":1,"qty":"0.01","quoteQty":"600","commission":"0","commissionAsset":"BTC","time":%d,"isBuyer":true}]`, trd.UnixMilli())
			} else {
				fmt.Fprint(w, `[]`)
			}
		case "/api/v3/klines":
			from, _ := strconv.ParseInt(q.Get("startTime"), 10, 64)
			to, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
			if q.Get("endTime") == "" { // как у Binance: без endTime отдаёт limit свечей от startTime
				lim, _ := strconv.ParseInt(q.Get("limit"), 10, 64)
				to = (from+59999)/60000*60000 + (lim-1)*60000
			}
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

func newBinanceServer(t *testing.T, fake *httptest.Server) (*Server, *db.Storage) {
	t.Helper()
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := NewServer(st, nil, nil)
	cl := binance.NewClient("K", "S")
	cl.SetBaseURL(fake.URL)
	cl.Sleep = func(time.Duration) {}
	s.SetBinance(cl)
	return s, st
}

func crawlBinanceOnce(s *Server) {
	s.prog.begin()
	s.crawlBinance()
	s.prog.finish()
}

// Binance обновляется ровно одним путём — «Разведать» (crawlBinance); GET /api/binance только читает кеш.
func TestBinanceViaCrawlerIntoCache(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	dep, trd := now.Add(-3*time.Hour), now.Add(-2*time.Hour)
	fake := newHistFake(dep, trd)
	defer fake.Close()
	s, st := newBinanceServer(t, fake)
	s.SetBinanceHistoryFrom(now.Add(-24 * time.Hour))

	// до опроса: GET ничего не опрашивает и ничего не создаёт
	if c, b := call(t, s.HandleBinance, "GET", "/api/binance", ""); c != 200 || !strings.Contains(string(b), `"points":0`) {
		t.Fatalf("до синка: %d %s", c, b)
	}
	if c, _ := call(t, s.HandleBinance, "POST", "/api/binance", ""); c != http.StatusMethodNotAllowed {
		t.Fatalf("POST больше нет: %d", c)
	}

	crawlBinanceOnce(s)

	// панель: строка на каждую статью (депозиты … цены), у каждой честный таймлайн; всё готово
	v := s.progressSnapshot()
	if len(v.Clients) != 1 || v.Clients[0].Kind != "binance" || len(v.Clients[0].Accounts) != len(rowStages) {
		t.Fatalf("панель: %+v", v.Clients)
	}
	byLabel := map[string]accView{}
	for _, a := range v.Clients[0].Accounts {
		byLabel[a.Label] = a
		if a.State != "done" || a.Percent != 100 || strings.Contains(a.Cells, "0") {
			t.Fatalf("строка %s: %+v", a.Label, a)
		}
	}
	// депозит найден — в строке депозитов есть зелёная клетка, в выводах (пусто) только косые
	if d := byLabel["Депозиты"]; !strings.Contains(d.Cells, "1") {
		t.Fatalf("депозиты без зелёной клетки: %s", d.Cells)
	}
	if w := byLabel["Выводы"]; strings.Contains(w.Cells, "1") {
		t.Fatalf("выводов не было, а клетка зелёная: %s", w.Cells)
	}
	// минутные цены идут по реальному периоду (с первого движения), а не по горизонту поиска
	if p := byLabel["Минутные цены"]; p.AxisFrom != dep.Unix() || !strings.Contains(p.Cells, "1") || !strings.Contains(p.Cells, "2") {
		t.Fatalf("цены: from=%d (ждали %d) cells=%s", p.AxisFrom, dep.Unix(), p.Cells)
	}

	// счёт — обычный счёт кеша
	c, b := call(t, s.HandleGetAccounts, "GET", "/api/accounts", "")
	if c != 200 || !strings.Contains(string(b), `"bank_key":"binance"`) || !strings.Contains(string(b), `"type":"binance"`) {
		t.Fatalf("accounts: %d %s", c, b)
	}
	// История по минутам с первого депозита; разбор по монетам — в кеше
	if first, _, n := st.BinanceSeriesInfo(); first != dep.Unix() || n < 180 {
		t.Fatalf("ряд: first=%d n=%d", first, n)
	}
	_, b = call(t, s.HandleBinance, "GET", "/api/binance", "")
	var gv struct {
		Configured bool `json:"configured"`
		Detail     struct {
			Total   float64 `json:"total"`
			History struct {
				Flows int `json:"flows"`
			} `json:"history"`
		} `json:"detail"`
	}
	json.Unmarshal(b, &gv)
	// 400 USDT + 0.01 BTC × 60000 = 1000 $ × 40 = 40000 ₴
	if !gv.Configured || gv.Detail.Total != 40000 || gv.Detail.History.Flows == 0 {
		t.Fatalf("GET: %s", b)
	}

	// свечи: последняя корзина — текущая оценка
	hour := now.Truncate(time.Hour)
	body := fmt.Sprintf(`{"starts":[%d,%d,%d],"end":%d}`, hour.Add(-4*time.Hour).Unix(), hour.Add(-time.Hour).Unix(), hour.Unix(), now.Unix()+60)
	c, b = call(t, s.HandleBinanceCandles, "POST", "/api/binance/candles", body)
	var cv struct {
		Has       bool         `json:"has"`
		LastValue float64      `json:"last_value"`
		Candles   [][4]float64 `json:"candles"`
	}
	json.Unmarshal(b, &cv)
	if c != 200 || !cv.Has || cv.LastValue != 40000 || len(cv.Candles) != 3 || cv.Candles[2][3] != 40000 {
		t.Fatalf("свечи: %d %s", c, b)
	}

	// второй «Разведать»: ряд уже есть — только дописывается, история не пересобирается (flows сохранены)
	crawlBinanceOnce(s)
	if accs := s.progressSnapshot().Clients[0].Accounts; len(accs) != 1 || accs[0].AxisFrom != dep.Unix() || !strings.Contains(accs[0].Cells, "1") {
		t.Fatalf("при дописывании одна строка, ось и клетки из кеша: %+v", s.progressSnapshot().Clients[0].Accounts)
	}
	_, b = call(t, s.HandleBinance, "GET", "/api/binance", "")
	var gv2 struct {
		Detail struct {
			History struct {
				Flows int `json:"flows"`
			} `json:"history"`
		} `json:"detail"`
	}
	json.Unmarshal(b, &gv2)
	if gv2.Detail.History.Flows != gv.Detail.History.Flows {
		t.Fatalf("итог истории потерян при дописывании: %s", b)
	}
	if first, _, _ := st.BinanceSeriesInfo(); first != dep.Unix() {
		t.Fatalf("начало ряда сдвинулось: %d", first)
	}

	// вайп кеша сносит Binance целиком; следующий «Разведать» восстанавливает
	if err := st.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if _, _, n := st.BinanceSeriesInfo(); n != 0 {
		t.Fatal("после вайпа ряд остался")
	}
	crawlBinanceOnce(s)
	if first, _, n := st.BinanceSeriesInfo(); first != dep.Unix() || n < 180 {
		t.Fatalf("после вайпа не восстановилось: first=%d n=%d", first, n)
	}
}

func TestBinanceNoMovementsStartsFromNow(t *testing.T) {
	fake := newHistFake(time.Time{}, time.Time{}) // ни депозитов, ни сделок
	defer fake.Close()
	s, st := newBinanceServer(t, fake)
	s.SetBinanceHistoryFrom(time.Now().Add(-time.Hour))
	crawlBinanceOnce(s)
	accs, _ := st.GetAccounts()
	// 400 USDT + 0.01 BTC × 60000 = 1000 $ × 40 = 40000 ₴
	if len(accs) != 1 || accs[0].Balance != 40000 {
		t.Fatalf("счета: %+v", accs)
	}
	if _, _, n := st.BinanceSeriesInfo(); n != 1 {
		t.Fatalf("ряд должен начаться с сегодняшней оценки: n=%d", n)
	}
}

func TestBinanceNoKey(t *testing.T) {
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)
	if c, b := call(t, s.HandleBinance, "GET", "/api/binance", ""); c != 200 || !strings.Contains(string(b), `"configured":false`) {
		t.Fatalf("без ключа: %d %s", c, b)
	}
	if err := s.syncBinance(nil); err == nil || !strings.Contains(err.Error(), "не подключён") {
		t.Fatalf("синк без ключа: %v", err)
	}
}

// crawlBank — общий обход для всех банков: счета сохраняются, ошибка info даёт ошибку клиента в панели.
func TestCrawlBankGeneric(t *testing.T) {
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)
	s.prog.begin()
	s.crawlBank(bankSpec{
		kind: "raif", num: 1, total: 1, tag: "RAIF",
		info: func() (string, []mono.AccountDTO, error) { return "", nil, errors.New("boom") },
	})
	s.prog.finish()
	v := s.progressSnapshot()
	if len(v.Clients) != 1 || v.Clients[0].Name != "Raif 1" || v.Clients[0].Error == "" {
		t.Fatalf("ошибка ключа: %+v", v.Clients)
	}
	s.crawlBank(bankSpec{
		kind: "mono", num: 1, total: 1,
		info: func() (string, []mono.AccountDTO, error) { return "Ivan", nil, nil },
	})
	if accs, _ := st.GetAccounts(); len(accs) != 0 {
		t.Fatalf("пустой ответ банка: %+v", accs)
	}
}

// Каждый просмотренный период сразу в кеше: упавший посреди синк продолжается, а не качает всё заново; дублей нет.
func TestBinanceResumesFromCachedWindows(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	dep, trd := now.Add(-3*time.Hour), now.Add(-2*time.Hour)
	var depReq atomic.Int64
	var broken atomic.Bool
	broken.Store(true)
	fake := newHistFake(dep, trd, func(path string) bool {
		if path == "/sapi/v1/capital/deposit/hisrec" {
			depReq.Add(1)
		}
		return broken.Load() && path == "/api/v3/exchangeInfo" // сбой после всех поисков движений
	})
	defer fake.Close()
	s, st := newBinanceServer(t, fake)
	s.SetBinanceHistoryFrom(now.Add(-400 * 24 * time.Hour)) // 5 окон депозитов по 89 суток

	crawlBinanceOnce(s)
	first := depReq.Load()
	if first < 5 {
		t.Fatalf("ожидали ≥5 окон депозитов, было %d", first)
	}
	if _, _, n := st.BinanceSeriesInfo(); n != 0 {
		t.Fatal("после сбоя ряда быть не должно")
	}
	flows, _ := st.BinanceFlows()
	if len(flows) == 0 || !st.BinanceCovered("депозиты", "", now.Add(-400*24*time.Hour).UnixMilli(), now.Add(-400*24*time.Hour).Add(89*24*time.Hour).UnixMilli()) {
		t.Fatalf("периоды и движения должны лежать в кеше после сбоя: %d", len(flows))
	}
	// сбой посреди синка: счёт уже в кеше (самым первым шагом), депозит уже операцией счёта — до сборки ряда
	if all, _ := st.GetAccounts(); len(all) != 1 || all[0].ID != "binance" || all[0].Balance != 40000 {
		t.Fatalf("счёт Binance должен быть в кеше сразу: %+v", all)
	}
	var early int
	allTx, _ := st.GetAllTransactions(100)
	for _, x := range allTx {
		if x.AccountID == "binance" && x.Description == "Binance: депозит 1000 USDT" && x.Amount == 40000 {
			early++
		}
	}
	if early != 1 {
		t.Fatalf("депозит должен лежать в истории операций сразу после периода: %+v", allTx)
	}
	accs := s.progressSnapshot().Clients[0].Accounts
	if accs[0].State != "done" || accs[4].State != "done" || accs[5].State != "error" || accs[7].State != "queued" {
		t.Fatalf("после сбоя пройденные строки готовы, упавшая — ошибка, остальные в очереди: %+v", accs)
	}

	broken.Store(false)
	depReq.Store(0)
	crawlBinanceOnce(s)
	if got := depReq.Load(); got > 1 {
		t.Fatalf("при продолжении запрашивается только хвост (было %d, стало %d)", first, got)
	}
	// итог такой же, как у чистого прогона: движения не задвоились
	_, b := call(t, s.HandleBinance, "GET", "/api/binance", "")
	var gv struct {
		Detail struct {
			History struct {
				Flows int `json:"flows"`
			} `json:"history"`
		} `json:"detail"`
	}
	json.Unmarshal(b, &gv)
	if gv.Detail.History.Flows != 3 { // депозит + две ноги покупки BTC за USDT
		t.Fatalf("движений должно быть 3, получено %d: %s", gv.Detail.History.Flows, b)
	}
	// депозит записан операцией счёта Binance (сумма — скачок оценки в ₴)
	txs, _ := st.GetAllTransactions(100)
	var found bool
	for _, x := range txs {
		if x.AccountID == "binance" && strings.Contains(x.Description, "депозит 1000 USDT") && x.Amount == 40000 {
			found = true
		}
	}
	if !found {
		t.Fatalf("депозит должен быть операцией счёта Binance: %+v", txs)
	}
}

func TestBinanceRemainingForETA(t *testing.T) {
	s := NewServer(nil, nil, nil)
	now := time.Now().Unix()
	row := &accProg{Custom: true, State: "queued", Step: 89 * 86400, Mult: 1}
	if av := s.customAccView(row, now); av.remaining < 20 { // горизонт с 2020 года — десятки окон
		t.Fatalf("оценка запросов для ETA: %d", av.remaining)
	}
	row.State, row.Left = "fetch", 7
	if av := s.customAccView(row, now); av.remaining != 7 {
		t.Fatalf("явный счётчик: %d", av.remaining)
	}
	row.State = "done"
	if av := s.customAccView(row, now); av.remaining != 0 {
		t.Fatalf("готовая строка: %d", av.remaining)
	}
}

// Лимит Binance (429 + Retry-After) виден в панели как «остывание» с периодом запроса, а после паузы строка снова качает.
func TestBinanceCoolingShownInPanel(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	var hit atomic.Bool
	fake := newHistFake(now.Add(-3*time.Hour), now.Add(-2*time.Hour), func(path string) bool { return false })
	defer fake.Close()
	// обёртка: первый запрос конвертаций получает 429
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sapi/v1/convert/tradeFlow" && hit.CompareAndSwap(false, true) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(429)
			return
		}
		resp, err := http.Get(fake.URL + r.URL.RequestURI())
		if err != nil {
			w.WriteHeader(502)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}))
	defer proxy.Close()
	s, _ := newBinanceServer(t, proxy)
	s.SetBinanceHistoryFrom(now.Add(-24 * time.Hour))
	var seen *accView
	s.bn.client.Sleep = func(d time.Duration) {
		if d < time.Second || seen != nil {
			return
		}
		for _, a := range s.progressSnapshot().Clients[0].Accounts {
			if a.State == "cooling" {
				x := a
				seen = &x
			}
		}
	}
	crawlBinanceOnce(s)
	if seen == nil {
		t.Fatal("во время паузы лимита ни одна строка не была в состоянии cooling")
	}
	if seen.Label != "Конвертации" || seen.CoolUntil <= time.Now().Unix() || seen.Win == nil {
		t.Fatalf("остывание: %+v", seen)
	}
	for _, a := range s.progressSnapshot().Clients[0].Accounts {
		if a.State != "done" {
			t.Fatalf("после паузы всё должно завершиться: %+v", a)
		}
	}
}

// Сделки идут по парам, а не по времени: пока идут, ось не красится косыми, видны лишь найденные сделки; процент — по парам.
func TestTradesRowIsNotFilledByPairs(t *testing.T) {
	s := NewServer(nil, nil, nil)
	now := time.Now().Unix()
	row := &accProg{Custom: true, Ahead: true, State: "fetch", Pct: 14, AxisFrom: now - 1000000, FlowAt: []int64{now - 500000}}
	av := s.customAccView(row, now)
	if av.Percent != 14 || strings.Contains(av.Cells, "2") || strings.Count(av.Cells, "1") != 1 {
		t.Fatalf("в процессе: %d %s", av.Percent, av.Cells)
	}
	row.State = "done"
	av = s.customAccView(row, now)
	if strings.Contains(av.Cells, "0") || strings.Count(av.Cells, "1") != 1 {
		t.Fatalf("готово: %s", av.Cells)
	}
}

// Заливка окон растёт от «сейчас» к началу истории (как у банков): справа зелёное/косое, слева ещё не просмотрено.
func TestBinanceFillGrowsRightToLeft(t *testing.T) {
	s := NewServer(nil, nil, nil)
	now := time.Now().Unix()
	lo := now - 1000*86400
	row := &accProg{Custom: true, State: "fetch", AxisFrom: lo, Cov: now - 500*86400, Step: 89 * 86400, Mult: 1}
	av := s.customAccView(row, now)
	n := len(av.Cells)
	if av.Percent < 49 || av.Percent > 51 || av.Cells[0] != '0' || av.Cells[n-1] != '2' || strings.Count(av.Cells, "0") < n/2-2 {
		t.Fatalf("справа просмотрено, слева нет: %d%% %s", av.Percent, av.Cells)
	}
	if av.remaining < 5 || av.remaining > 6 { // осталось ≈ 500 суток слева: 6 окон по 89
		t.Fatalf("осталось окон: %d", av.remaining)
	}
}

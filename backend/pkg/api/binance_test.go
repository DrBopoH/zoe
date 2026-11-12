package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Публичный Binance: последние цены, пары и минутные свечи. Ключей и подписанных запросов больше нет.
func newPriceFake(spy ...func(path string) (status int, retryAfter string)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, f := range spy {
			if st, ra := f(r.URL.Path); st != 0 {
				if ra != "" {
					w.Header().Set("Retry-After", ra)
				}
				w.WriteHeader(st)
				return
			}
		}
		q := r.URL.Query()
		switch r.URL.Path {
		case "/api/v3/ticker/price":
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","price":"60000"},{"symbol":"USDTUAH","price":"40"}]`)
		case "/api/v3/exchangeInfo":
			fmt.Fprint(w, `{"symbols":[{"symbol":"BTCUSDT","baseAsset":"BTC","quoteAsset":"USDT"},{"symbol":"USDTUAH","baseAsset":"USDT","quoteAsset":"UAH"}]}`)
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
	cl := binance.NewClient()
	cl.SetBaseURL(fake.URL)
	cl.Sleep = func(time.Duration) {}
	s.SetBinance(cl)
	return s, st
}

func crawlBinanceOnce(s *Server) {
	s.prog.begin()
	s.crawlBinance(context.Background())
	s.prog.finish()
}

const stmtHead = "Ідентифікатор користувача,Час,Акаунт,Операція,Монета,Змінити,Примітка\n"

// Выписка: депозит 1000 USDT, покупка 0.01 BTC за 600 USDT. Остаток 400 USDT + 0.01 BTC = 1000 $ = 40000 ₴.
func stmtFor(dep, trd time.Time) string {
	f := func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") }
	return stmtHead +
		"7," + f(dep) + ",Spot,Deposit,USDT,1000,\n" +
		"7," + f(trd) + ",Spot,Transaction Buy,BTC,0.01,\n" +
		"7," + f(trd) + ",Spot,Transaction Spend,USDT,-600,\n"
}

const stmtName = "Binance-(UTC+0).csv"

// Binance — такой же путь, как у банков: выписка через общий импорт → кеш; «Разведать» строит оценку из журнала.
func TestBinanceStatementThenCrawlerIntoCache(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	dep, trd := now.Add(-3*time.Hour), now.Add(-2*time.Hour)
	fake := newPriceFake()
	defer fake.Close()
	s, st := newBinanceServer(t, fake)

	// до выписки: ничего нет, «Разведать» по Binance ничего не делает
	if c, b := call(t, s.HandleBinance, "GET", "/api/binance", ""); c != 200 || !strings.Contains(string(b), `"points":0`) || !strings.Contains(string(b), `"loaded":false`) {
		t.Fatalf("до выписки: %d %s", c, b)
	}
	if err := s.syncBinance(context.Background(), nil); err != nil {
		t.Fatalf("без выписки синк — не ошибка: %v", err)
	}

	res, err := s.importSource(stmtName, []byte(stmtFor(dep, trd)))
	if err != nil || res.Count != 3 || res.Card != "binance" || res.AccountID != db.BinanceAccountID {
		t.Fatalf("импорт: %+v %v", res, err)
	}
	if maskPAN("binance") != "binance" {
		t.Fatalf("папка источников: %s", maskPAN("binance"))
	}
	// счёт уже есть, оценки ещё нет (баланс 0, ряда нет)
	c, b := call(t, s.HandleGetAccounts, "GET", "/api/accounts", "")
	if c != 200 || !strings.Contains(string(b), `"bank_key":"binance"`) || !strings.Contains(string(b), `"name":"`+stmtName+` (binance)"`) {
		t.Fatalf("accounts: %d %s", c, b)
	}
	if _, _, n := st.BinanceSeriesInfo(); n != 0 {
		t.Fatal("ряд строится при «Разведать», не при импорте")
	}

	crawlBinanceOnce(s)

	v := s.progressSnapshot()
	if len(v.Clients) != 1 || v.Clients[0].Kind != "binance" || len(v.Clients[0].Accounts) != 1 {
		t.Fatalf("панель: %+v", v.Clients)
	}
	if row := v.Clients[0].Accounts[0]; row.State != "done" || row.Percent != 100 || row.AxisFrom != dep.Unix() || !strings.Contains(row.Cells, "1") {
		t.Fatalf("строка Binance: %+v", row)
	}
	if first, _, n := st.BinanceSeriesInfo(); first != dep.Unix() || n < 180 {
		t.Fatalf("ряд с первого движения: first=%d n=%d", first, n)
	}
	_, b = call(t, s.HandleBinance, "GET", "/api/binance", "")
	var gv struct {
		Loaded bool `json:"loaded"`
		Detail struct {
			Total float64 `json:"total"`
			Items []struct {
				Asset string  `json:"asset"`
				Spot  float64 `json:"spot"`
			} `json:"items"`
			History struct {
				Flows int `json:"flows"`
			} `json:"history"`
		} `json:"detail"`
	}
	json.Unmarshal(b, &gv)
	if !gv.Loaded || gv.Detail.Total != 40000 || gv.Detail.History.Flows != 3 || len(gv.Detail.Items) != 2 {
		t.Fatalf("GET: %s", b)
	}
	// внешний депозит стал операцией счёта: 1000 USDT × 40 = 40000 ₴
	txs, _ := st.GetAllTransactions(10)
	if len(txs) != 1 || txs[0].AccountID != db.BinanceAccountID || txs[0].Amount != 40000 || !txs[0].Internal {
		t.Fatalf("операции: %+v", txs)
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

	// второй «Разведать»: ряд есть — только дописывается; операции не задваиваются
	crawlBinanceOnce(s)
	if first, _, _ := st.BinanceSeriesInfo(); first != dep.Unix() {
		t.Fatalf("начало ряда сдвинулось: %d", first)
	}
	if txs, _ := st.GetAllTransactions(10); len(txs) != 1 {
		t.Fatalf("операции задвоились: %+v", txs)
	}

	// повторная загрузка того же файла журнал не задваивает, ряд сбрасывается до пересборки
	if _, err := s.importSource(stmtName, []byte(stmtFor(dep, trd))); err != nil {
		t.Fatal(err)
	}
	if rows, _ := st.BinanceLedger(); len(rows) != 3 {
		t.Fatalf("журнал: %d строк", len(rows))
	}
	if _, _, n := st.BinanceSeriesInfo(); n != 0 {
		t.Fatal("после выписки ряд сброшен до пересборки")
	}

	// вайп кеша сносит Binance целиком; «Загрузить» и «Разведать» восстанавливают
	if err := st.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if st.HasBinance() {
		t.Fatal("после вайпа журнал остался")
	}
	if _, err := s.importSource(stmtName, []byte(stmtFor(dep, trd))); err != nil {
		t.Fatal(err)
	}
	crawlBinanceOnce(s)
	if first, _, n := st.BinanceSeriesInfo(); first != dep.Unix() || n < 180 {
		t.Fatalf("после вайпа не восстановилось: first=%d n=%d", first, n)
	}
}

func TestBinanceBrokenStatementIsRejected(t *testing.T) {
	fake := newPriceFake()
	defer fake.Close()
	s, st := newBinanceServer(t, fake)
	_, err := s.importSource(stmtName, []byte(stmtHead+"7,не время,Spot,Deposit,USDT,1,\n"))
	var ie *importError
	if !errors.As(err, &ie) || ie.code != http.StatusBadRequest || st.HasBinance() {
		t.Fatalf("битая выписка: %v", err)
	}
}

// Лимит Binance (429 + Retry-After) виден в панели как «остывание», а после паузы строка снова качает и завершается.
func TestBinanceCoolingShownInPanel(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	var hit atomic.Bool
	fake := newPriceFake(func(path string) (int, string) {
		if path == "/api/v3/klines" && hit.CompareAndSwap(false, true) {
			return 429, "7"
		}
		return 0, ""
	})
	defer fake.Close()
	s, _ := newBinanceServer(t, fake)
	if _, err := s.importSource(stmtName, []byte(stmtFor(now.Add(-3*time.Hour), now.Add(-2*time.Hour)))); err != nil {
		t.Fatal(err)
	}
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
		t.Fatal("во время паузы лимита строка не была в состоянии cooling")
	}
	if seen.CoolUntil <= time.Now().Unix() {
		t.Fatalf("остывание: %+v", seen)
	}
	if a := s.progressSnapshot().Clients[0].Accounts[0]; a.State != "done" {
		t.Fatalf("после паузы всё должно завершиться: %+v", a)
	}
}

// Заливка таймлайна растёт слева направо по проценту; клетка с движением журнала зелёная, пустая — косая.
func TestBinanceRowTimeline(t *testing.T) {
	s := NewServer(nil, nil, nil)
	now := time.Now().Unix()
	row := &accProg{Custom: true, State: "fetch", Pct: 50, AxisFrom: now - 1000000, FlowAt: []int64{now - 1000000}, Left: 4}
	av := s.customAccView(row, now)
	n := len(av.Cells)
	if av.Percent != 50 || av.Cells[0] != '1' || av.Cells[1] != '2' || av.Cells[n-1] != '0' || av.remaining != 4 {
		t.Fatalf("в процессе: %d%% %s rem=%d", av.Percent, av.Cells, av.remaining)
	}
	row.State = "done"
	if av = s.customAccView(row, now); strings.Contains(av.Cells, "0") || av.remaining != 0 {
		t.Fatalf("готово: %s rem=%d", av.Cells, av.remaining)
	}
}

func TestCrawlBankGeneric(t *testing.T) {
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)
	s.prog.begin()
	s.crawlBank(context.Background(), bankSpec{
		kind: "raif", num: 1, total: 1, tag: "RAIF",
		info: func() (string, []mono.AccountDTO, error) { return "", nil, errors.New("boom") },
	})
	s.prog.finish()
	v := s.progressSnapshot()
	if len(v.Clients) != 1 || v.Clients[0].Name != "Raif 1" || v.Clients[0].Error == "" {
		t.Fatalf("ошибка ключа: %+v", v.Clients)
	}
	s.crawlBank(context.Background(), bankSpec{
		kind: "mono", num: 1, total: 1,
		info: func() (string, []mono.AccountDTO, error) { return "Ivan", nil, nil },
	})
	if accs, _ := st.GetAccounts(); len(accs) != 0 {
		t.Fatalf("пустой ответ банка: %+v", accs)
	}
}

// Баланс счёта совпадает с последней точкой минутного ряда, даже если «последняя цена» из тикера чуть отличается:
// график считает остальной капитал как «итог − конец ряда», расхождение сдвинуло бы его (в «Откр» первой свечи).
func TestBinanceBalanceEqualsSeriesEnd(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/api/v3/ticker/price":
			fmt.Fprint(w, `[{"symbol":"USDTUAH","price":"40.4"}]`) // тикер ушёл от минутных свечей
		case "/api/v3/exchangeInfo":
			fmt.Fprint(w, `{"symbols":[{"symbol":"USDTUAH","baseAsset":"USDT","quoteAsset":"UAH"}]}`)
		case "/api/v3/klines":
			from, _ := strconv.ParseInt(q.Get("startTime"), 10, 64)
			to, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
			var rows []string
			for ts := (from + 59999) / 60000 * 60000; ts <= to && len(rows) < 1000; ts += 60000 {
				rows = append(rows, fmt.Sprintf(`[%d,"0","0","0","40","0",0,"0",0,"0","0","0"]`, ts))
			}
			fmt.Fprint(w, "["+strings.Join(rows, ",")+"]")
		default:
			w.WriteHeader(404)
		}
	}))
	defer fake.Close()
	s, st := newBinanceServer(t, fake)
	now := time.Now().UTC().Truncate(time.Minute)
	stmt := stmtHead + "7," + now.Add(-3*time.Hour).Format("2006-01-02 15:04:05") + ",Spot,Deposit,USDT,100,\n"
	if _, err := s.importSource(stmtName, []byte(stmt)); err != nil {
		t.Fatal(err)
	}
	crawlBinanceOnce(s)
	accs, _ := st.GetAllAccounts()
	_, last, ok := st.BinanceLastMinute()
	if !ok || len(accs) != 1 || accs[0].Balance != last || last != 4000 {
		t.Fatalf("баланс %v, конец ряда %v (ждали 4000, а не 4040 по тикеру)", accs[0].Balance, last)
	}
}

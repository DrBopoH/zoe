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

const stmtSample = "\ufeffІдентифікатор користувача,Час,Акаунт,Операція,Монета,Змінити,Примітка\n" +
	"1,2026-05-24 07:49:32,Spot,Transfer Between Spot and Strategy,USDT,-80.5,\n" +
	"1,2026-05-24 07:49:32,Strategy,Transfer Between Spot and Strategy,USDT,80.5,\n" + // сторона Strategy — не наша
	"1,2026-05-24 08:00:00,Spot,Transaction Buy,BTC,0.001,\n" + // сделки уже приходят из API
	"1,2026-05-25 08:00:00,Spot,Simple Earn Flexible Interest,USDT,0.1,\n" + // и Earn тоже
	"1,2026-05-29 01:25:02,USD-M Futures,Fee,USDT,-1,\n" +
	"1,2026-06-01 10:00:00,Spot,Small Assets Exchange BNB,FIDA,-0.09,\n" +
	"1,2026-06-01 10:00:00,Spot,Small Assets Exchange BNB,BNB,0.001,\n" +
	"1,2026-06-02 10:00:00,Spot,SOL Staking - Purchase,SOL,-0.2,\n"

func TestStatementKeepsOnlyWhatAPIMisses(t *testing.T) {
	loc := StatementZone("Binance-Історія-транзакцій-202611111427(UTC+2)-part1-of1.csv")
	fl, total, err := ParseStatement(strings.NewReader(stmtSample), loc)
	if err != nil {
		t.Fatal(err)
	}
	if total != 8 || len(fl) != 4 {
		t.Fatalf("строк %d, движений %d: %+v", total, len(fl), fl)
	}
	if fl[0].Kind != "strategy" || fl[0].Asset != "USDT" || fl[0].Delta != -80.5 {
		t.Fatalf("перевод в Strategy: %+v", fl[0])
	}
	if want := time.Date(2026, 5, 24, 5, 49, 32, 0, time.UTC).UnixMilli(); fl[0].T != want { // UTC+2 → UTC
		t.Fatalf("время: %d != %d", fl[0].T, want)
	}
	if fl[1].Kind != "stmt:dust" || fl[3].Kind != "stmt:staking" {
		t.Fatalf("виды: %+v", fl)
	}
}

func TestStatementDetectionAndZone(t *testing.T) {
	if !IsStatement([]byte(stmtSample)) || !IsStatement([]byte("User_ID,UTC_Time,Account,Operation,Coin,Change,Remark\n")) {
		t.Fatal("выписку Binance нужно узнавать (укр. и англ. заголовок, BOM)")
	}
	if IsStatement([]byte("Дата,Опис,Сума\n")) || IsStatement([]byte("%PDF-1.4")) {
		t.Fatal("банковские файлы не должны узнаваться как Binance")
	}
	if _, _, err := ParseStatement(strings.NewReader("a,b\n"), nil); err == nil {
		t.Fatal("чужой файл — ошибка")
	}
	if StatementZone("x.csv") != time.UTC {
		t.Fatal("без пометки — UTC")
	}
	if _, off := time.Date(2026, 1, 1, 0, 0, 0, 0, StatementZone("a(UTC-5:30)b")).Zone(); off != -(5*3600 + 1800) {
		t.Fatalf("смещение: %d", off)
	}
}

// Выписка не дублирует API: событие «stmt:X» отбрасывается, если рядом по времени есть X из API; иначе берётся как X.
func TestMergeStatementDropsWhatAPIAlreadyHas(t *testing.T) {
	fl := []Flow{
		{T: 10_000, Asset: "USDT", Delta: -20, Kind: "futures"},      // из API
		{T: 10_400, Asset: "USDT", Delta: -20, Kind: "stmt:futures"}, // то же событие из выписки (секунды vs мс)
		{T: 900_000, Asset: "USDT", Delta: 5, Kind: "stmt:futures"},  // API этого не показал (старше 6 месяцев)
		{T: 50_000, Asset: "FIDA", Delta: -1, Kind: "stmt:dust"},     // пыли из API нет вовсе
		{T: 60_000, Asset: "USDT", Delta: -80, Kind: "strategy"},     // у Strategy API нет: остаётся как есть
	}
	got := mergeStatement(fl)
	if len(got) != 4 {
		t.Fatalf("ожидали 4 движения: %+v", got)
	}
	kinds := map[int64]string{}
	for _, f := range got {
		kinds[f.T] = f.Kind
	}
	if kinds[10_400] != "" || kinds[900_000] != "futures" || kinds[50_000] != "dust" || kinds[60_000] != "strategy" {
		t.Fatalf("после слияния: %+v", got)
	}
}

// Источники API для фьючерсов, пыли и стейкинга: разбор ответов и знак движений; границы глубины истории.
func TestTransferDustStakingFetchers(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	at := now.Add(-48 * time.Hour).UnixMilli()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		st, _ := strconv.ParseInt(q.Get("startTime"), 10, 64)
		en, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
		if r.URL.Path != "/sapi/v1/time" && (at < st || at > en) { // событие отдаётся только окну, в которое оно попадает
			fmt.Fprint(w, `{"total":0,"rows":[],"userAssetDribblets":[]}`)
			return
		}
		switch r.URL.Path {
		case "/sapi/v1/asset/transfer":
			if q.Get("type") == "MAIN_UMFUTURE" {
				fmt.Fprintf(w, `{"total":2,"rows":[{"asset":"USDT","amount":"20","type":"MAIN_UMFUTURE","status":"CONFIRMED","tranId":1,"timestamp":%d},{"asset":"USDT","amount":"7","type":"MAIN_UMFUTURE","status":"FAILED","tranId":2,"timestamp":%d}]}`, at, at)
			} else {
				fmt.Fprintf(w, `{"total":1,"rows":[{"asset":"USDT","amount":"3","type":"UMFUTURE_MAIN","status":"CONFIRMED","tranId":3,"timestamp":%d}]}`, at+1000)
			}
		case "/sapi/v1/asset/dribblet":
			fmt.Fprintf(w, `{"total":1,"userAssetDribblets":[{"operateTime":%d,"userAssetDribbletDetails":[{"amount":"0.09","transferedAmount":"0.001","fromAsset":"FIDA","targetAsset":"BNB","operateTime":%d}]}]}`, at, at)
		case "/sapi/v1/sol-staking/sol/history/stakingHistory":
			fmt.Fprintf(w, `{"total":1,"rows":[{"time":%d,"asset":"SOL","amount":"0.2","distributeAsset":"BNSOL","distributeAmount":"0.177","status":"SUCCESS"}]}`, at)
		case "/sapi/v1/sol-staking/sol/history/redemptionHistory":
			fmt.Fprint(w, `{"total":0,"rows":[]}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := NewClient("K", "S")
	c.SetBaseURL(srv.URL)
	c.Sleep = func(time.Duration) {}
	from := now.Add(-2000 * day)

	sum := func(fl []Flow, kind, asset string) (n int, v float64) {
		for _, f := range fl {
			if f.Kind == kind && f.Asset == asset {
				n++
				v += f.Delta
			}
		}
		return
	}
	fut, err := c.FuturesTransfers(from, now)
	if n, v := sum(fut, "futures", "USDT"); err != nil || n != 2 || v != -17 { // −20 туда, +3 обратно; FAILED не считаем
		t.Fatalf("фьючерсы: %+v %v", fut, err)
	}
	dust, err := c.Dust(from, now)
	if n, v := sum(dust, "dust", "FIDA"); err != nil || n != 1 || v != -0.09 {
		t.Fatalf("пыль: %+v %v", dust, err)
	}
	if _, v := sum(dust, "dust", "BNB"); v != 0.001 {
		t.Fatalf("BNB от пыли: %+v", dust)
	}
	st, err := c.SolStaking(from, now)
	if _, v := sum(st, "staking", "SOL"); err != nil || v != -0.2 {
		t.Fatalf("стейкинг: %+v %v", st, err)
	}
	if _, v := sum(st, "staking", "BNSOL"); v != 0.177 {
		t.Fatalf("BNSOL: %+v", st)
	}

	// глубина истории: фьючерсы — 6 месяцев, пыль — с 2020-12-01, SOL — с 2024; сетка остаётся от общего начала
	if s := StageStart("фьючерсы", from, now); s.Before(now.Add(-futuresBack)) || s.After(now.Add(-futuresBack).Add(29*day)) || s.Sub(from)%(29*day) != 0 {
		t.Fatalf("старт фьючерсов: %v", s)
	}
	if s := StageStart("пыль", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), now); s.Before(dustSince) {
		t.Fatalf("старт пыли: %v", s)
	}
	if !StageStart("депозиты", from, now).IsZero() {
		t.Fatal("у депозитов ограничений нет")
	}
}

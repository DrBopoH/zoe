package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Фикстуры собраны по документации НБУ и FiscalData (живой ответ при написании недоступен), данные вымышленные.
const nbuRegistry = `[
 {"cpcode":"UA4000000001","cpdescr":"ОВДП тест 2030","val_code":"UAH","nominal":1000,"auk_proc":"16,44","razm_date":"2024-02-21","pgs_date":"20.02.2030"},
 {"cpcode":"UA4000000002","cpdescr":"ОВДП USD","val_code":"USD","nominal":"1000","auk_proc":4.5,"pgs_date":"2027-05-05T00:00:00"},
 {"cpcode":"UA4000000003","cpdescr":"Бескупонная","val_code":"UAH","auk_proc":null,"pgs_date":"2026-12-01"},
 {"cpcode":"UA1000000009","cpdescr":"Корпоративная","val_code":"UAH","auk_proc":20,"pgs_date":"2028-01-01"},
 {"cpdescr":"без ISIN"}
]`

func newNBU(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/depo_securities", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(nbuRegistry)) })
	mux.HandleFunc("/NBU_ovdp", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("date") {
		case "10.11.2026": // старый аукцион
			w.Write([]byte(`[{"stockcode":"UA4000000001","valcode":"UAH","repaydate":"20.02.2030","auc_proc":"15,9","auctiondate":"10.11.2026"}]`))
		case "12.11.2026": // новый перетирает старый; ещё одна бумага вне реестра
			w.Write([]byte(`[{"stockcode":"UA4000000001","valcode":"UAH","auc_proc":"16,1","auctiondate":"12.11.2026"},
			                 {"stockcode":"UA4000000009","valcode":"UAH","repaydate":"01.03.2031","auc_proc":15,"auctiondate":"12.11.2026"},
			                 {"stockcode":"XX","valcode":"UAH"}]`))
		default:
			w.Write([]byte(`[]`)) // день без аукциона — пустой массив
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testClient(nbu, ust string) *Client {
	c := NewClient()
	c.NBUBase, c.USTBase = nbu, ust
	c.EurostatBase = "" // инфляция еврозоны в этих тестах не опрашивается
	c.Now = func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) }
	c.AuctionDays = 5
	return c
}

func TestNBURegistryFiltersOVDPAndParsesFields(t *testing.T) {
	srv := newNBU(t)
	got, res, err := testClient(srv.URL, "").NBURegistry(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || res.Fetched != 3 || res.Skipped != 2 {
		t.Fatalf("ожидали 3 ОВДП и 2 пропуска: %d %+v", len(got), res)
	}
	a := got[0]
	if a.ID != "UA4000000001" || a.CouponRate != 16.44 || a.Maturity != "2030-02-20" || a.Issued != "2024-02-21" || a.Nominal != 1000 || a.Currency != "UAH" {
		t.Fatalf("разбор строки: %+v", a)
	}
	if b := got[1]; b.Currency != "USD" || b.Maturity != "2027-05-05" || b.CouponRate != 4.5 {
		t.Fatalf("USD-бумага: %+v", b)
	}
	if c := got[2]; c.CouponRate != 0 || c.Nominal != 1000 {
		t.Fatalf("бескупонная: %+v", c)
	}
}

func TestSyncNBUMergesRegistryAndLatestAuction(t *testing.T) {
	srv := newNBU(t)
	got, res := testClient(srv.URL, "").SyncNBU(context.Background(), nil)
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	by := map[string]Security{}
	for _, s := range got {
		by[s.ID] = s
	}
	a := by["UA4000000001"]
	if a.Yield != 16.1 || a.LastAuction != "2026-11-12" || a.CouponRate != 16.44 {
		t.Fatalf("последний аукцион должен победить, условия — из реестра: %+v", a)
	}
	n := by["UA4000000009"]
	if n.Maturity != "2031-03-01" || n.Yield != 15 || n.Nominal != 1000 {
		t.Fatalf("бумага только из аукциона: %+v", n)
	}
	if len(got) != 4 {
		t.Fatalf("реестр 3 + 1 новая из аукциона = 4, а %d", len(got))
	}
}

func TestSyncNBUReportsGrowingProgress(t *testing.T) {
	srv := newNBU(t)
	var pcts []int
	var notes []string
	testClient(srv.URL, "").SyncNBU(context.Background(), func(p int, n string) { pcts, notes = append(pcts, p), append(notes, n) })
	if len(pcts) < 3 || pcts[0] != 0 || notes[0] != "реестр" {
		t.Fatalf("первым — реестр: %v %v", pcts, notes)
	}
	for i := 1; i < len(pcts); i++ {
		if pcts[i] < pcts[i-1] || pcts[i] > 100 {
			t.Fatalf("процент должен расти и не выходить за 100: %v", pcts)
		}
	}
	if !strings.HasPrefix(notes[len(notes)-1], "аукционы: ") {
		t.Fatalf("последняя заметка — день аукциона: %v", notes)
	}
}

func TestSyncNBUOneHalfFailsKeepsOther(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/depo_securities", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(nbuRegistry)) })
	mux.HandleFunc("/NBU_ovdp", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	got, res := testClient(srv.URL, "").SyncNBU(context.Background(), nil)
	if len(got) != 3 || !strings.Contains(res.Error, "HTTP 500") {
		t.Fatalf("реестр должен остаться, ошибка аукционов — в Result: %d %q", len(got), res.Error)
	}
}

func TestUSTAuctionsDedupAndFields(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		if r.URL.Path != "/v1/accounting/od/auctions_query" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"data":[
		 {"cusip":"912810XX1","security_type":"Bond","security_term":"30-Year","auction_date":"2026-11-05","issue_date":"2026-11-15","maturity_date":"2056-11-15","int_rate":"4.625","high_yield":"4.701"},
		 {"cusip":"912810XX1","security_type":"Bond","security_term":"30-Year","auction_date":"2026-08-07","issue_date":"2026-08-15","maturity_date":"2056-11-15","int_rate":"4.625","high_yield":"4.9"},
		 {"cusip":"912797ZZ9","security_type":"Bill","security_term":"13-Week","auction_date":"2026-11-10","issue_date":"2026-11-13","maturity_date":"2027-02-12","int_rate":"0.000","high_yield":"null","high_investment_rate":"3.95"},
		 {"security_type":"Note"}]}`))
	}))
	defer srv.Close()
	got, res, err := testClient("", srv.URL).USTAuctions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || res.Skipped != 1 {
		t.Fatalf("дубль CUSIP и запись без CUSIP не должны попасть: %d %+v", len(got), res)
	}
	if a := got[0]; a.Yield != 4.701 || a.CouponRate != 4.625 || a.Name != "30-Year Bond" || a.Currency != "USD" || a.Nominal != 100 || a.LastAuction != "2026-11-05" {
		t.Fatalf("самый поздний аукцион должен победить: %+v", a)
	}
	if b := got[1]; b.CouponRate != 0 || b.Yield != 3.95 || b.Maturity != "2027-02-12" {
		t.Fatalf("bill: %+v", b)
	}
	if !strings.Contains(gotQuery, "auction_date%3Agte%3A2023-11-12") || !strings.Contains(gotQuery, "page%5Bsize%5D=10000") {
		t.Fatalf("параметры запроса: %s", gotQuery)
	}
}

func TestBadResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`<html>not json`)) }))
	defer srv.Close()
	c := testClient(srv.URL, srv.URL)
	if _, _, err := c.NBURegistry(context.Background()); err == nil {
		t.Fatal("не-JSON должен дать ошибку")
	}
	if _, _, err := c.USTAuctions(context.Background()); err == nil {
		t.Fatal("не-JSON должен дать ошибку")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := c.NBUAuctions(ctx, nil); err == nil {
		t.Fatal("отменённый контекст должен остановить обход")
	}
}

func TestNormDate(t *testing.T) {
	for in, want := range map[string]string{"2030-02-20": "2030-02-20", "20.02.2030": "2030-02-20", "20300220": "2030-02-20", "2030-02-20T00:00:00": "2030-02-20", "": "", "x": ""} {
		if got := normDate(in); got != want {
			t.Errorf("%q → %q, ждали %q", in, got, want)
		}
	}
}

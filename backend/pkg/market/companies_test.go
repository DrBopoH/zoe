package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const secFixture = `{"fields":["cik","name","ticker","exchange"],"data":[[320193,"Apple Inc.","AAPL","Nasdaq"],[1318605,"Tesla, Inc.","TSLA","Nasdaq"],[1067983,"BERKSHIRE HATHAWAY INC","BRK-B","NYSE"],[1067983,"BERKSHIRE HATHAWAY INC","BRK-A","NYSE"],[999,"Фонд без цены","FUND","NYSE"],[5,null,"BAD",null],[7,"Dup","AAPL","OTC"]]}`

const nasdaqFixture = `{"data":{"asOf":null,"headers":{},"rows":[
{"symbol":"TSLA","name":"Tesla, Inc. Common Stock","lastsale":"$420.50","marketCap":"1300000000000.00","country":"United States","ipoyear":"2010","sector":"Consumer Discretionary","industry":"Auto Manufacturers"},
{"symbol":"AAPL","name":"Apple Inc. Common Stock","lastsale":"$ 3,070.00","marketCap":"4500000000000.00","country":"United States","ipoyear":"1980","sector":"Technology","industry":"Computer Manufacturing"},
{"symbol":"BRK/B","name":"Berkshire Hathaway Inc.","lastsale":"$500.00","marketCap":"1100000000000.00","country":"United States","ipoyear":"","sector":"Finance","industry":"Property-Casualty"},
{"symbol":"AAPL","name":"Дубль","lastsale":"$1.00","marketCap":"10.00","country":"","ipoyear":"","sector":"","industry":""},
{"symbol":"FUND","name":"Нет цены","lastsale":"N/A","marketCap":"","country":"","ipoyear":"","sector":"","industry":""},
{"symbol":"NOSEC","name":"Не в реестре SEC","lastsale":"$10.00","marketCap":"1000000.00","country":"","ipoyear":"","sector":"","industry":""}
]}}`

func companiesServer(t *testing.T, secStatus int) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/files/company_tickers_exchange.json":
			if ua := r.Header.Get("User-Agent"); ua != secUserAgent || !strings.Contains(ua, "@") || strings.Contains(ua, "localhost") || strings.ContainsAny(ua, "()") {
				http.Error(w, "нужен User-Agent", http.StatusForbidden)
				return
			}
			if secStatus != 0 {
				http.Error(w, "down", secStatus)
				return
			}
			w.Write([]byte(secFixture))
		case "/api/screener/stocks":
			if r.Header.Get("User-Agent") != "Mozilla/5.0" {
				http.Error(w, "нужен браузерный User-Agent", http.StatusForbidden)
				return
			}
			w.Write([]byte(nasdaqFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	c := NewClient()
	c.NasdaqBase, c.SECBase = srv.URL, srv.URL
	c.Retries = 0
	return c
}

func TestCompaniesJoinNasdaqWithSEC(t *testing.T) {
	list, res := companiesServer(t, 0).Companies(context.Background(), nil)
	if res.Error != "" || res.Source != SourceCompanies {
		t.Fatalf("итог: %+v", res)
	}
	if len(list) != 3 || res.Fetched != 3 || res.Skipped != 3 {
		t.Fatalf("3 компании, 3 строки пропущены (дубль, без цены, без пары в SEC): %+v %+v", list, res)
	}
	// по капитализации: Apple, Tesla, Berkshire
	if list[0].Ticker != "AAPL" || list[1].Ticker != "TSLA" || list[2].Ticker != "BRK-B" {
		t.Fatalf("порядок: %+v", list)
	}
	a := list[0]
	if a.CIK != 320193 || a.Exchange != "Nasdaq" || a.Price != 3070 || a.MarketCap != 4.5e12 || a.IPOYear != 1980 || a.Sector != "Technology" || a.Country != "United States" {
		t.Fatalf("Apple: %+v", a)
	}
	if b := list[2]; b.Ticker != "BRK-B" || b.CIK != 1067983 || b.IPOYear != 0 {
		t.Fatalf("класс акций BRK/B сошёлся с BRK-B из SEC: %+v", b)
	}
}

func TestCompaniesFailureGivesNoHalfList(t *testing.T) {
	list, res := companiesServer(t, http.StatusServiceUnavailable).Companies(context.Background(), nil)
	if len(list) != 0 || !strings.HasPrefix(res.Error, "SEC: ") {
		t.Fatalf("SEC лежит — списка нет: %d %+v", len(list), res)
	}
	c := companiesServer(t, 0)
	c.NasdaqBase = c.SECBase + "/нет"
	if list, res = c.Companies(context.Background(), nil); len(list) != 0 || !strings.HasPrefix(res.Error, "Nasdaq: ") {
		t.Fatalf("Nasdaq недоступен: %d %+v", len(list), res)
	}
	c.NasdaqBase = ""
	if list, res = c.Companies(context.Background(), nil); list != nil || res.Error != "" {
		t.Fatalf("пустой адрес отключает опрос: %+v %+v", list, res)
	}
}

func TestTickerKeyAndMoney(t *testing.T) {
	for in, want := range map[string]string{"BRK/B": "BRK-B", "bf.b": "BF-B", " AAPL ": "AAPL", "A B": "A-B"} {
		if got := tickerKey(in); got != want {
			t.Errorf("tickerKey(%q)=%q, ждали %q", in, got, want)
		}
	}
	for in, want := range map[string]float64{"$1,234.50": 1234.5, "$ 3,070.00": 3070, "": 0, "N/A": 0, "-5": 0, "NaN": 0, "12": 12} {
		if got := moneyNum(in); got != want {
			t.Errorf("moneyNum(%q)=%v, ждали %v", in, got, want)
		}
	}
}

func TestCompaniesFeedIsRegistered(t *testing.T) {
	if !IsSource(SourceCompanies) {
		t.Fatal("источник companies должен быть известен маршрутам")
	}
	var f *Feed
	for _, x := range NewClient().Feeds() {
		if x.ID == SourceCompanies {
			x := x
			f = &x
		}
	}
	if f == nil || f.Companies == nil || f.Fetch != nil || f.Series != nil || f.Unit != "компаний" {
		t.Fatalf("фид: %+v", f)
	}
	if _, ok := NewClient().Catalog()[SourceCompanies]; ok {
		t.Fatal("у списка компаний нет рядов в каталоге")
	}
}

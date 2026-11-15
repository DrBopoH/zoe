package market

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// chainServer — один сервер за все адреса контура; fail — часть пути, на которую отвечать ошибкой.
func chainServer(t *testing.T, fail string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail != "" && strings.Contains(r.URL.Path, fail) {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.Contains(r.URL.Path, "adv/search"):
			// первое объявление нерабочее (остаток на 100 ₴), дальше рабочие: цены 47,0 47,1 47,2 (медиана 47,1); продажа — 46,0 46,2 46,1
			if strings.Contains(string(body), `"BUY"`) {
				w.Write([]byte(`{"data":[{"adv":{"price":"40.00","tradableQuantity":"2.5","minSingleTransAmount":"50","maxSingleTransAmount":"100"}},` +
					`{"adv":{"price":"47.00","tradableQuantity":"1000","minSingleTransAmount":"1000","maxSingleTransAmount":"50000"}},` +
					`{"adv":{"price":"47.20","tradableQuantity":"1000","minSingleTransAmount":"1000","maxSingleTransAmount":"50000"}},` +
					`{"adv":{"price":"47.10","tradableQuantity":"1000","minSingleTransAmount":"1000","maxSingleTransAmount":"50000"}}]}`))
			} else {
				w.Write([]byte(`{"data":[{"adv":{"price":"46.00","tradableQuantity":"1000","minSingleTransAmount":"20000","maxSingleTransAmount":"50000"}},` +
					`{"adv":{"price":"46.00","tradableQuantity":"1000","minSingleTransAmount":"500","maxSingleTransAmount":"5000"}},` +
					`{"adv":{"price":"46.20","tradableQuantity":"1000","minSingleTransAmount":"500","maxSingleTransAmount":"50000"}}]}`))
			}
		case strings.Contains(r.URL.Path, "fundingRate"):
			// сутки 2026-11-10: три выплаты по 0,0001 → среднее 0,0001 × 3 × 365 × 100 = 10,95 % годовых
			w.Write([]byte(`[{"fundingTime":1794614400000,"fundingRate":"0.00010000"},{"fundingTime":1794643200000,"fundingRate":"0.00010000"},{"fundingTime":1794672000000,"fundingRate":"0.00010000"},{"fundingTime":1794700800002,"fundingRate":"-0.00020000"}]`))
		case strings.Contains(r.URL.Path, "openInterestHist"):
			w.Write([]byte(`[{"sumOpenInterestValue":"7225625852.83","timestamp":1794528000000},{"sumOpenInterestValue":"bad","timestamp":1794614400000}]`))
		case strings.Contains(r.URL.Path, "stablecoincharts"):
			w.Write([]byte(`[{"date":"1794009600","totalCirculatingUSD":{"peggedUSD":170000000000}},{"date":"1794096000","totalCirculatingUSD":{"peggedUSD":171000000000}},{"date":"1794700800","totalCirculatingUSD":{"peggedUSD":175362324347.73}},{"date":"1","totalCirculatingUSD":{"peggedUSD":0}}]`))
		case strings.Contains(r.URL.Path, "fee-rates"):
			w.Write([]byte(`[{"timestamp":1794700800,"avgFee_50":1},{"timestamp":1794704400,"avgFee_50":3}]`))
		case strings.Contains(r.URL.Path, "fees/recommended"):
			w.Write([]byte(`{"halfHourFee":10}`))
		case strings.Contains(r.URL.Path, "getchainparameters"):
			w.Write([]byte(`{"chainParameter":[{"key":"getTransactionFee","value":1000},{"key":"getEnergyFee","value":100}]}`))
		case r.Method == http.MethodPost && strings.Contains(string(body), "eth_gasPrice"):
			w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x3b9aca00"}`)) // 1 гвей
		case strings.Contains(r.URL.Path, "ticker/price"):
			switch r.URL.Query().Get("symbol") {
			case "BTCUSDT":
				w.Write([]byte(`{"price":"60000.00"}`))
			case "TRXUSDT":
				w.Write([]byte(`{"price":"0.30"}`))
			case "ETHUSDT":
				w.Write([]byte(`{"price":"2000.00"}`))
			}
		case strings.Contains(r.URL.Path, "/global"):
			w.Write([]byte(`{"data":{"market_cap_percentage":{"btc":58.123,"eth":10},"total_market_cap":{"usd":2205249867927.15}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func chainClient(srv *httptest.Server) *Client {
	c := NewClient()
	c.BinanceBase, c.P2PBase, c.FuturesBase, c.LlamaBase, c.MempoolBase, c.TronBase, c.EthRPCBase, c.GeckoBase = srv.URL, srv.URL, srv.URL, srv.URL, srv.URL, srv.URL, srv.URL, srv.URL
	c.Now = func() time.Time { return time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC) }
	return c
}

func TestChainBuildsAllSeries(t *testing.T) {
	pts, res := chainClient(chainServer(t, "")).Chain(context.Background(), nil)
	if res.Error != "" || res.Source != SourceChain || res.Fetched != len(pts) {
		t.Fatalf("%+v", res)
	}
	by := byID(pts)
	for _, id := range chainSeries {
		if len(by[id]) == 0 {
			t.Errorf("нет ряда %s", id)
		}
	}
	// P2P: нерабочее объявление пропущено, медиана 47,00 47,10 47,20 → 47,10; продажа: объявление с минимумом 20 000 ₴ и с максимумом 5 000 ₴ не подходят → одно рабочее 46,20
	if a := by[SerP2PAsk]; len(a) != 1 || a[0].Value != 47.1 || a[0].Date != "2026-11-16" {
		t.Fatalf("ask: %+v", a)
	}
	if b := by[SerP2PBid]; len(b) != 1 || b[0].Value != 46.2 {
		t.Fatalf("bid: %+v", b)
	}
	// Фандинг: сутки 2026-11-12 — три выплаты по 0,0001 → 10,95 % годовых; следующие сутки — одна выплата −0,0002 → −21,9
	f := by[SerFundBTC]
	if len(f) != 2 || f[0].Value != 10.95 || f[1].Value != -21.9 {
		t.Fatalf("funding: %+v", f)
	}
	if o := by[SerOIBTC]; len(o) != 1 || o[0].Value != 7.23 {
		t.Fatalf("oi: %+v", o)
	}
	// USDT: три точки одной недели → последняя; нулевая пропущена
	if s := by[SerStableUSDT]; len(s) == 0 || s[len(s)-1].Value != 175.362 {
		t.Fatalf("usdt: %+v", s)
	}
	// BTC: 10 сат/vB × 140 vB = 1400 сат = 0,000014 BTC × 60 000 = 0,84 $; медиана блоков за сутки (1 и 3 → 2)
	if v := by[SerFeeBTCUSD]; len(v) != 1 || v[0].Value != 0.84 {
		t.Fatalf("btc fee: %+v", v)
	}
	if v := by[SerFeeBTCSat]; len(v) != 1 || v[0].Value != 2 {
		t.Fatalf("btc sat: %+v", v)
	}
	// TRON: (65 000 × 100 + 345 × 1000) / 1e6 = 6,845 TRX × 0,30 = 2,0535 $
	if v := by[SerFeeTronUSD]; len(v) != 1 || v[0].Value != 2.0535 {
		t.Fatalf("tron fee: %+v", v)
	}
	// ETH: 65 000 газа × 1 гвей = 0,000065 ETH × 2000 = 0,13 $
	if v := by[SerFeeEthUSD]; len(v) != 1 || v[0].Value != 0.13 {
		t.Fatalf("eth fee: %+v", v)
	}
	if d := by[SerDomBTC]; len(d) != 1 || d[0].Value != 58.12 {
		t.Fatalf("dominance: %+v", d)
	}
	if m := by[SerMcapTotal]; len(m) != 1 || m[0].Value != 2.205 {
		t.Fatalf("mcap: %+v", m)
	}
}

func TestChainOneFailureKeepsRest(t *testing.T) {
	for _, tc := range []struct{ fail, lost, kept string }{
		{"adv/search", SerP2PAsk, SerFundBTC},
		{"fundingRate", SerFundBTC, SerP2PAsk},
		{"stablecoincharts", SerStableUSDT, SerFeeEthUSD},
		{"getchainparameters", SerFeeTronUSD, SerFeeBTCUSD},
		{"/global", SerDomBTC, SerOIBTC},
	} {
		pts, res := chainClient(chainServer(t, tc.fail)).Chain(context.Background(), nil)
		by := byID(pts)
		if res.Error == "" || len(by[tc.lost]) != 0 || len(by[tc.kept]) == 0 {
			t.Errorf("%s: упал только %s: %q %v", tc.fail, tc.lost, res.Error, len(by))
		}
	}
}

func TestChainWithoutBasesPollsNothing(t *testing.T) {
	c := NewClient()
	c.P2PBase, c.FuturesBase, c.LlamaBase, c.MempoolBase, c.TronBase, c.EthRPCBase, c.GeckoBase = "", "", "", "", "", "", ""
	if pts, res := c.Chain(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("%d %q", len(pts), res.Error)
	}
}

func TestP2PRejectsEmptyAds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"data":[]}`)) }))
	defer srv.Close()
	c := chainClient(srv)
	if _, err := c.p2pPrice(context.Background(), "BUY"); err == nil {
		t.Fatal("пустая выдача — ошибка, а не нулевая цена")
	}
}

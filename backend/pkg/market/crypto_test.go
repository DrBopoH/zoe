package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cryptoServer(t *testing.T, failSymbol string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/klines":
			sym := r.URL.Query().Get("symbol")
			if sym == failSymbol {
				http.Error(w, "x", http.StatusBadGateway)
				return
			}
			if r.URL.Query().Get("interval") != "1w" {
				t.Errorf("interval=%q", r.URL.Query().Get("interval"))
			}
			// 2026-11-02 и 2026-11-09 (понедельники); третья строка короткая и нулевая цена — пропускаются
			w.Write([]byte(`[[1793577600000,"100","110","90","105.5","1",1794182399999],[1794182400000,"105","120","100","118","1",1794787199999],[1],[1794787200000,"1","1","1","0","1",1]]`))
		case "/fng/":
			if r.URL.Query().Get("limit") != "0" {
				t.Errorf("limit=%q", r.URL.Query().Get("limit"))
			}
			// дневные точки двух недель, значение строкой и числом; одна битая
			w.Write([]byte(`{"data":[{"value":"20","timestamp":"1794009600"},{"value":25,"timestamp":"1794096000"},{"value":"x","timestamp":"1794182400"},{"value":"70","timestamp":"1794441600"}]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func cryptoClient(srv *httptest.Server) *Client {
	c := NewClient()
	c.BinanceBase, c.FNGBase = srv.URL, srv.URL
	return c
}

func TestCryptoBuildsSeries(t *testing.T) {
	pts, res := cryptoClient(cryptoServer(t, "")).Crypto(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	btc := by[SerCryptoBTC]
	if len(btc) != 2 || btc[0].Date != "2026-11-02" || btc[0].Value != 105.5 || btc[1].Value != 118 {
		t.Fatalf("btc: %+v", btc)
	}
	for _, s := range []string{SerCryptoETH, SerCryptoBNB, SerCryptoSOL} {
		if len(by[s]) != 2 {
			t.Fatalf("%s: %+v", s, by[s])
		}
	}
	fng := by[SerCryptoFNG]
	if len(fng) < 2 || fng[0].Value != 25 {
		t.Fatalf("fng (неделя → последнее значение, битая пропущена): %+v", fng)
	}
	if res.Skipped < 8+1 { // по 2 пропуска у каждой из 4 монет + битая точка индекса
		t.Fatalf("skipped=%d", res.Skipped)
	}
}

func TestCryptoFailedCoinDoesNotKillOthers(t *testing.T) {
	pts, res := cryptoClient(cryptoServer(t, "ETHUSDT")).Crypto(context.Background(), nil)
	by := byID(pts)
	if len(by[SerCryptoETH]) != 0 || len(by[SerCryptoBTC]) == 0 || len(by[SerCryptoFNG]) == 0 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "ETHUSDT") {
		t.Fatalf("error: %q", res.Error)
	}
}

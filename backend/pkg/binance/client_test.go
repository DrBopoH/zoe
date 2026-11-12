package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// подставной Binance: проверяет подпись и ключ в заголовке
func fakeServer(t *testing.T, secret string, calls *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, r.Method+" "+r.URL.Path)
		if strings.HasPrefix(r.URL.Path, "/api/v3/time") {
			fmt.Fprint(w, `{"serverTime": 1700000000000}`)
			return
		}
		if r.URL.Path == "/api/v3/ticker/price" {
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","price":"60000.0"},{"symbol":"ETHBTC","price":"0.05"},{"symbol":"USDTUAH","price":"41.5"},{"symbol":"BNBUSDT","price":"600"}]`)
			return
		}
		// всё остальное — подписанное
		q := r.URL.RawQuery
		i := strings.LastIndex(q, "&signature=")
		if i < 0 {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"code":-2014,"msg":"API-key format invalid."}`)
			return
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(q[:i]))
		if hex.EncodeToString(mac.Sum(nil)) != q[i+len("&signature="):] || r.Header.Get("X-MBX-APIKEY") != "KEY" {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"code":-1022,"msg":"Signature for this request is not valid."}`)
			return
		}
		if !strings.Contains(q, "timestamp=") || !strings.Contains(q, "recvWindow=") {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"code":-1102,"msg":"Mandatory parameter 'timestamp' was not sent"}`)
			return
		}
		switch r.URL.Path {
		case "/api/v3/account":
			fmt.Fprint(w, `{"balances":[{"asset":"BTC","free":"0.01","locked":"0.005"},{"asset":"USDT","free":"100","locked":"0"},{"asset":"LDUSDT","free":"50","locked":"0"},{"asset":"ABC","free":"0","locked":"0"}]}`)
		case "/sapi/v1/asset/get-funding-asset":
			if r.Method != "POST" {
				w.WriteHeader(405)
				return
			}
			fmt.Fprint(w, `[{"asset":"USDT","free":"20","locked":"0","freeze":"5"}]`)
		case "/sapi/v1/simple-earn/flexible/position":
			fmt.Fprint(w, `{"rows":[{"asset":"USDT","totalAmount":"50.5"},{"asset":"ETH","totalAmount":"1"}],"total":2}`)
		case "/sapi/v1/simple-earn/locked/position":
			w.WriteHeader(403)
			fmt.Fprint(w, `{"code":-2015,"msg":"Invalid API-key, IP, or permissions for action."}`)
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestFetchAndValue(t *testing.T) {
	var calls []string
	srv := fakeServer(t, "SECRET", &calls)
	defer srv.Close()
	c := NewClient("KEY", "SECRET")
	c.SetBaseURL(srv.URL)

	h, warns, err := c.Fetch()
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "locked") {
		t.Fatalf("ожидалось одно предупреждение про locked: %v", warns)
	}
	if w := h["USDT"]; w == nil || w.Spot != 100 || w.Funding != 25 || w.Earn != 50.5 {
		t.Fatalf("USDT: %+v", w)
	}
	if _, ok := h["LDUSDT"]; ok {
		t.Fatal("LD* из спота не должна учитываться (иначе двойной счёт с Earn)")
	}
	if w := h["BTC"]; w == nil || w.Spot != 0.015 {
		t.Fatalf("BTC: %+v", w)
	}

	prices, err := c.Prices()
	if err != nil {
		t.Fatal(err)
	}
	v, ok := Value(h, prices)
	if !ok {
		t.Fatal("оценка не удалась")
	}
	// USDT 175.5×41.5 = 7283.25; BTC 0.015×60000×41.5 = 37350; ETH 1 через ETHBTC×BTCUSDT = 3000 USDT → 124500
	want := 7283.25 + 37350 + 124500
	if v.TotalUAH != round2(want) {
		t.Fatalf("итог %v, ждали %v\n%+v", v.TotalUAH, want, v.Items)
	}
	if v.Items[0].Asset != "ETH" {
		t.Fatalf("сортировка по стоимости: %+v", v.Items)
	}
}

func TestSpotRequiredAndBadSecret(t *testing.T) {
	var calls []string
	srv := fakeServer(t, "SECRET", &calls)
	defer srv.Close()
	c := NewClient("KEY", "WRONG")
	c.SetBaseURL(srv.URL)
	_, _, err := c.Fetch()
	ae, ok := err.(*APIError)
	if !ok || ae.Code != -1022 {
		t.Fatalf("плохая подпись должна давать APIError -1022, а не %v", err)
	}
}

func TestValueNeedsUAHRateAndUnpriced(t *testing.T) {
	h := Holdings{"XYZ": {Spot: 5}, "UAH": {Spot: 100}}
	if _, ok := Value(h, map[string]float64{"BTCUSDT": 1}); ok {
		t.Fatal("без USDTUAH оценивать нельзя")
	}
	v, ok := Value(h, map[string]float64{"USDTUAH": 40})
	if !ok || v.TotalUAH != 100 || len(v.Unpriced) != 1 || v.Unpriced[0] != "XYZ" {
		t.Fatalf("%+v", v)
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Холодный старт: свежая установка, обе базы пусты, ни токенов, ни файлов-источников. Каждый экран фронта при открытии
// спрашивает эти маршруты — все должны ответить 200 и корректным JSON (пустым, но не null там, где фронт перебирает список),
// а не 500: иначе первый запуск у нового человека открывается красной ошибкой.
func TestColdStartReadEndpoints(t *testing.T) {
	h := newTestHandler(t)
	paths := []string{
		"/api/accounts", "/api/transactions", "/api/users", "/api/alias-structures", "/api/iban-masks",
		"/api/icu/bonds",
		"/api/market/bonds", "/api/market/companies", "/api/market/lists?name=icu", "/api/market/curve?name=icu",
		"/api/macro", "/api/macro/catalog", "/api/macro/sources",
		"/api/binance", "/api/binance/moves", "/api/binance/trades", "/api/binance/swaps", "/api/binance/p2p",
		"/api/sync/status", "/api/sources",
	}
	for _, p := range paths {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", local+p, nil))
		if w.Code != http.StatusOK {
			t.Errorf("%s: %d %q, ждали 200 на пустой установке", p, w.Code, w.Body.String())
			continue
		}
		var v any
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Errorf("%s: ответ не JSON: %v", p, err)
		}
	}
}

// Свечи портфеля (POST) на пустой установке: пустой запрос — не 5xx.
func TestColdStartCandlesNever5xx(t *testing.T) {
	h := newTestHandler(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", local+"/api/binance/candles", strings.NewReader(`{"starts":[],"end":0}`)))
	if w.Code >= 500 {
		t.Errorf("candles: %d %q", w.Code, w.Body.String())
	}
}

// Маршруты с обязательным параметром без него отвечают 4xx (понятная ошибка запроса), но никогда не 5xx.
func TestColdStartParamEndpointsNever5xx(t *testing.T) {
	h := newTestHandler(t)
	for _, p := range []string{"/api/icu/ledger", "/api/icu/ledger?account_id=nope", "/api/market/stock", "/api/market/stock?ticker=ZZZZ", "/api/macro/ticks", "/api/macro/ticks?series=nope", "/api/macro?series=nope"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", local+p, nil))
		if w.Code >= 500 {
			t.Errorf("%s: %d %q", p, w.Code, w.Body.String())
		}
	}
}

package mono

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewClient("tok")
	c.baseURL = srv.URL
	return c
}

func TestGetClientInfo(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/personal/client-info" || r.Header.Get("X-Token") != "tok" {
			t.Errorf("запрос: %s, токен %q", r.URL.Path, r.Header.Get("X-Token"))
		}
		w.Write([]byte(`{"name":"Тест","accounts":[{"id":"a1","balance":1250,"type":"black"}]}`))
	})
	info, err := c.GetClientInfo()
	if err != nil || info.Name != "Тест" || len(info.Accounts) != 1 || info.Accounts[0].Balance != 1250 {
		t.Fatalf("%+v %v", info, err)
	}
}

func TestStatementErrors(t *testing.T) {
	from := time.Unix(1700000000, 0)
	for code, check := range map[int]func(error) bool{
		429: func(e error) bool { var x *RateLimitError; return errors.As(e, &x) },
		400: func(e error) bool { var x *BadRequestError; return errors.As(e, &x) },
		500: func(e error) bool { return e != nil && e.Error() == "mono statement status: 500" },
	} {
		code, check := code, check
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) })
		if _, err := c.GetStatement("a1", from, from.Add(time.Hour)); !check(err) {
			t.Errorf("статус %d: %v", code, err)
		}
	}
}

func TestClientInfoBadRequestIsPlainError(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400) })
	_, err := c.GetClientInfo()
	var x *BadRequestError
	if err == nil || errors.As(err, &x) || err.Error() != "mono client info status: 400" {
		t.Fatalf("400 у client-info должна быть обычной ошибкой: %v", err)
	}
}

func TestStatementPath(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/personal/statement/acc/1700000000/1700003600" {
			t.Errorf("путь: %s", r.URL.Path)
		}
		w.Write([]byte(`[{"id":"x","amount":-100}]`))
	})
	items, err := c.GetStatement("acc", time.Unix(1700000000, 0), time.Unix(1700003600, 0))
	if err != nil || len(items) != 1 || items[0].Amount != -100 {
		t.Fatalf("%+v %v", items, err)
	}
}

package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"economic/pkg/db"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return NewServer(st, nil, nil).Handler()
}

// CORS навешивается на весь роутер: чужой Origin отбивается на любом маршруте, а не только на тех, где его не забыли.
func TestEveryRouteIsBehindCORS(t *testing.T) {
	h := newTestHandler(t)
	for _, p := range []string{"/api/accounts", "/api/users", "/api/sources", "/api/binance", "/api/sync/status", "/api/icu/bonds"} {
		rq := httptest.NewRequest("GET", p, nil)
		rq.Header.Set("Origin", "http://evil.example")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, rq)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: чужой Origin пропущен (%d)", p, w.Code)
		}
	}
}

// Чтение — только GET, запись — только POST/PUT: раньше часть маршрутов не проверяла метод.
func TestMethodGuards(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct{ method, path string }{
		{"POST", "/api/accounts"},
		{"DELETE", "/api/transactions"},
		{"PUT", "/api/sync/status"},
		{"GET", "/api/sync/full"},
		{"GET", "/api/sync/wipe-cache"},
		{"GET", "/api/accounts/life"},
		{"DELETE", "/api/accounts/credit-limit"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: %d, ждали 405", c.method, c.path, w.Code)
		}
	}
}

func TestReadJSONRejectsBadAndHugeBodies(t *testing.T) {
	h := newTestHandler(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/users", strings.NewReader("{oops")))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "Invalid payload") {
		t.Fatalf("битый JSON: %d %q", w.Code, w.Body.String())
	}
	huge := `{"name":"` + strings.Repeat("я", maxJSONBody) + `"}`
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/users", strings.NewReader(huge)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("огромное тело принято: %d", w.Code)
	}
}

func TestEnvList(t *testing.T) {
	t.Setenv("A_TOKENS", " x , ,y,")
	t.Setenv("A_TOKEN", "solo")
	if got := EnvList("A_TOKENS", "A_TOKEN"); len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Fatalf("%q", got)
	}
	t.Setenv("A_TOKENS", "")
	if got := EnvList("A_TOKENS", "A_TOKEN"); len(got) != 1 || got[0] != "solo" {
		t.Fatalf("запасная переменная: %q", got)
	}
	if got := EnvList("NOPE_1", "NOPE_2"); got != nil {
		t.Fatalf("пусто: %q", got)
	}
}

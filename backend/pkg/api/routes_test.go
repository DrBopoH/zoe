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

// Адрес бэкенда в запросах тестов: Host проверяется на каждом маршруте (httptest по умолчанию подставляет example.com)
const local = "http://127.0.0.1:8080"

// CORS навешивается на весь роутер: чужой Origin отбивается на любом маршруте, а не только на тех, где его не забыли.
func TestEveryRouteIsBehindCORS(t *testing.T) {
	h := newTestHandler(t)
	for _, p := range []string{"/api/accounts", "/api/users", "/api/sources", "/api/binance", "/api/sync/status", "/api/icu/bonds"} {
		rq := httptest.NewRequest("GET", local+p, nil)
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
		h.ServeHTTP(w, httptest.NewRequest(c.method, local+c.path, nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: %d, ждали 405", c.method, c.path, w.Code)
		}
	}
}

func TestReadJSONRejectsBadAndHugeBodies(t *testing.T) {
	h := newTestHandler(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", local+"/api/users", strings.NewReader("{oops")))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "Invalid payload") {
		t.Fatalf("битый JSON: %d %q", w.Code, w.Body.String())
	}
	huge := `{"name":"` + strings.Repeat("я", maxJSONBody) + `"}`
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", local+"/api/users", strings.NewReader(huge)))
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

// DNS-rebinding: чужое имя, указывающее на 127.0.0.1, не должно доходить до хендлеров даже без Origin.
func TestHostHeaderAllowlist(t *testing.T) {
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := NewServer(st, nil, nil)
	s.SetAllowedOrigins([]string{"http://192.168.0.5:5173"})
	h := s.Handler()
	for host, want := range map[string]int{
		"127.0.0.1:8080":              http.StatusOK,
		"localhost:8080":              http.StatusOK,
		"LOCALHOST":                   http.StatusOK,
		"[::1]:8080":                  http.StatusOK,
		"192.168.0.5:8080":            http.StatusOK, // хост из ALLOWED_ORIGINS: доступ с другого устройства
		"evil.example":                http.StatusForbidden,
		"evil.example:8080":           http.StatusForbidden,
		"127.0.0.1.evil.example:8080": http.StatusForbidden,
		"":                            http.StatusForbidden,
	} {
		rq := httptest.NewRequest("GET", "/api/sync/status", nil)
		rq.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, rq)
		if w.Code != want {
			t.Errorf("Host %q: %d, ждали %d", host, w.Code, want)
		}
	}
}

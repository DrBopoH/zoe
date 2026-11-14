package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipJSON(t *testing.T) {
	h := gzipJSON(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("err") != "" {
			http.Error(w, "нет", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"a": strings.Repeat("x", 500)})
	})
	get := func(url, enc, method string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, url, nil)
		if enc != "" {
			req.Header.Set("Accept-Encoding", enc)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}

	rec := get("/x", "gzip, deflate", http.MethodGet)
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("JSON должен быть сжат: %v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(zr)
	if !strings.Contains(string(body), `"a":"xxx`) || rec.Body.Len() >= len(body) {
		t.Fatalf("тело после распаковки: %d байт сжатых против %d", rec.Body.Len(), len(body))
	}

	if rec := get("/x", "", http.MethodGet); rec.Header().Get("Content-Encoding") != "" || !strings.Contains(rec.Body.String(), `"a"`) {
		t.Fatal("клиент без gzip получает как раньше")
	}
	if rec := get("/x", "gzip", http.MethodPost); rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("сжимаем только GET")
	}
	rec = get("/x?err=1", "gzip", http.MethodGet)
	if rec.Header().Get("Content-Encoding") != "" || rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "нет") {
		t.Fatalf("текстовая ошибка не сжимается: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}
}

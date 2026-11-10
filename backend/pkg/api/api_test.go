package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"economic/pkg/db"
)

func TestHTTP(t *testing.T) {
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)

	// CORS
	h := s.EnableCORS(s.HandleGetAccounts)
	rq := httptest.NewRequest("GET", "/api/accounts", nil)
	rq.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	h(w, rq)
	if w.Code != 403 {
		t.Fatalf("evil origin: %d", w.Code)
	}
	rq.Header.Set("Origin", "http://localhost:5173")
	w = httptest.NewRecorder()
	h(w, rq)
	if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("ok origin: %d %v", w.Code, w.Header())
	}

	// импорт CSV
	csv := "Баланс на кінець періоду: 10,00\nUA583003350000000026001234567\nКартка 4149****7777\nДата і час здійснення операції;Деталі операції;MCC;Сума у валюті рахунку;Комісія\n10.03.2025 12:00:00;Переказ: Petro;4829;-54,10;0,00\n"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "Петро Іваненко.csv")
	fw.Write([]byte(csv))
	mw.Close()
	rq = httptest.NewRequest("POST", "/api/sync/import-csv", &buf)
	rq.Header.Set("Content-Type", mw.FormDataContentType())
	w = httptest.NewRecorder()
	s.HandleImportCSV(w, rq)
	if w.Code != 200 {
		t.Fatalf("import %d %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	t.Logf("import: %v", resp)

	// status отдаёт data_version, он растёт
	get := func() float64 {
		w := httptest.NewRecorder()
		s.HandleGetStatus(w, httptest.NewRequest("GET", "/api/sync/status", nil))
		var m map[string]any
		json.Unmarshal(w.Body.Bytes(), &m)
		return m["stats"].(map[string]any)["data_version"].(float64)
	}
	v1 := get()
	st.WipeCache()
	if get() <= v1 {
		t.Fatal("data_version must grow")
	}

	// пустое имя лица
	rq = httptest.NewRequest("POST", "/api/users", bytes.NewBufferString(`{"name":"  "}`))
	w = httptest.NewRecorder()
	s.HandleUsers(w, rq)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty name: %d", w.Code)
	}
}

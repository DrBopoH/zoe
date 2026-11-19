package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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
	rq := httptest.NewRequest("GET", "http://127.0.0.1:8080/api/accounts", nil)
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

// Дела: создание, список, правка, удаление и отказ на неверных полях.
func TestWorkspacesAPI(t *testing.T) {
	h := newTestHandler(t)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		rq := httptest.NewRequest(method, local+path, strings.NewReader(body))
		rq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, rq)
		return w
	}
	if w := do("GET", "/api/workspaces", ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("пустой список: %d %q", w.Code, w.Body.String())
	}
	w := do("POST", "/api/workspaces", `{"id":"hack","name":"Glovo","color":"#10b981","created_at":1}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"Glovo"`) || strings.Contains(w.Body.String(), `"id":"hack"`) {
		t.Fatalf("создание (id ставит сервер): %d %s", w.Code, w.Body.String())
	}
	var created struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("id в ответе: %v %s", err, w.Body.String())
	}
	if w := do("POST", "/api/workspaces", `{"name":" "}`); w.Code != 400 {
		t.Fatalf("пустое имя должно давать 400, а не %d", w.Code)
	}
	if w := do("POST", "/api/workspaces", `{"name":"x","color":"red"}`); w.Code != 400 {
		t.Fatalf("неверный цвет — 400, а не %d", w.Code)
	}
	if w := do("PUT", "/api/workspaces", `{"id":"nope","name":"x"}`); w.Code != 404 {
		t.Fatalf("правка чужого id — 404, а не %d", w.Code)
	}
	if w := do("PUT", "/api/workspaces", `{"id":"`+created.ID+`","name":"Glovo курьер"}`); w.Code != 200 {
		t.Fatalf("правка: %d %s", w.Code, w.Body.String())
	}
	if w := do("GET", "/api/workspaces", ""); !strings.Contains(w.Body.String(), "Glovo курьер") {
		t.Fatalf("список после правки: %s", w.Body.String())
	}
	if w := do("DELETE", "/api/workspaces", ""); w.Code != 400 {
		t.Fatalf("удаление без id — 400, а не %d", w.Code)
	}
	if w := do("DELETE", "/api/workspaces?id="+created.ID, ""); w.Code != 200 {
		t.Fatalf("удаление: %d", w.Code)
	}
	if w := do("GET", "/api/workspaces", ""); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("после удаления: %s", w.Body.String())
	}
	if w := do("PATCH", "/api/workspaces", ""); w.Code != 405 {
		t.Fatalf("PATCH — 405, а не %d", w.Code)
	}
}

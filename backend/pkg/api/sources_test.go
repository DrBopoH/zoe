package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const csvCard1 = `Виписка по картці
Рахунок: UA583003350000000026001234567
Картка: 4149****7777
Баланс на кінець періоду: 100,00
Дата і час здійснення операції;Номер картки;Категорія;Деталі операції;MCC;Сума у валюті рахунку;Комісія
10.03.2025 12:00:00;4149****7777;;Переказ з картки: Ivanenko Petro;4829;-54,10;0,00
11.03.2025 09:30:00;4149****7777;;Кава;5814;-54,10;0,00
`

// та же назва файла, другая карта и другое содержимое
const csvCard2 = `Виписка по картці
Рахунок: UA583003350000000026007654321
Картка: 4149****8888
Баланс на кінець періоду: 50,00
Дата і час здійснення операції;Номер картки;Категорія;Деталі операції;MCC;Сума у валюті рахунку;Комісія
12.03.2025 10:00:00;4149****8888;;Зарахування: Іван Петренко;4829;1000,00;0,00
`

func upload(t *testing.T, s *Server, name, content string) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write([]byte(content))
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/sync/import-csv", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.HandleImportCSV(rec, req)
	if rec.Code != 200 {
		t.Fatalf("import %s: %d %s", name, rec.Code, rec.Body.String())
	}
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func TestMaskPAN(t *testing.T) {
	for in, want := range map[string]string{
		"516875******8629": "516875******8629",
		"7778000000209689": "777800******9689",
		"4149****7777":     "4149****7777",
		"414951****5262":   "414951******5262",
		"":                 "unknown",
	} {
		if got := maskPAN(in); got != want {
			t.Errorf("maskPAN(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSourcesCacheFlow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECONOMIC_SOURCES_DIR", dir)
	e := newEnv(t)

	r1 := upload(t, e.s, "Петро Іваненко.csv", csvCard1)
	if r1["source_cached"] != true {
		t.Fatalf("new file must be cached: %v", r1)
	}
	// тот же файл под другим именем — по хэшу это дубль
	if r := upload(t, e.s, "copy (1).csv", csvCard1); r["source_cached"] != false {
		t.Fatalf("same content must not be cached twice: %v", r)
	}
	// то же имя, но другое содержимое и другая карта — новый источник в своей папке
	if r := upload(t, e.s, "Петро Іваненко.csv", csvCard2); r["source_cached"] != true {
		t.Fatalf("different content must be cached: %v", r)
	}

	for _, pan := range []string{"4149****7777", "4149****8888"} {
		m, _ := filepath.Glob(filepath.Join(dir, pan, "*"))
		if len(m) != 1 {
			t.Fatalf("folder %s: %v", pan, m)
		}
		if fi, _ := os.Stat(m[0]); fi.Mode().Perm() != 0o600 {
			t.Fatalf("source must be owner-only: %v", fi.Mode())
		}
		if !strings.HasSuffix(m[0], "Петро Іваненко.csv") && !strings.HasSuffix(m[0], "Петро Іваненко.csv") {
			t.Fatalf("name lost: %s", m[0])
		}
	}

	// сброс базы → «Загрузить» восстанавливает всё из источников, имя файла (алиас) сохраняется
	if err := e.st.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if txs, _ := e.st.GetTransactions(100); len(txs) != 0 {
		t.Fatal("wipe failed")
	}
	rec := httptest.NewRecorder()
	e.s.HandleLoadSources(rec, httptest.NewRequest(http.MethodPost, "/api/sources/load", nil))
	var res struct {
		Total, Loaded, Transactions int
		Failed                      []loadFailure
	}
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Total != 2 || res.Loaded != 2 || res.Transactions != 3 || len(res.Failed) != 0 {
		t.Fatalf("%+v body=%s", res, rec.Body.String())
	}
	txs, _ := e.st.GetTransactions(100)
	if len(txs) != 3 {
		t.Fatalf("restored %d", len(txs))
	}
	accs, _ := e.st.GetAccounts()
	for _, a := range accs {
		if !strings.HasPrefix(a.Name, "Петро Іваненко.csv") {
			t.Fatalf("alias from filename lost: %q", a.Name)
		}
	}

	// битый файл в кэше не ломает цикл
	os.WriteFile(filepath.Join(dir, "4149****7777", "00000000000000000000_broken.csv"), []byte("мусор"), 0o600)
	rec = httptest.NewRecorder()
	e.s.HandleLoadSources(rec, httptest.NewRequest(http.MethodPost, "/api/sources/load", nil))
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Total != 3 || res.Loaded != 2 || len(res.Failed) != 1 {
		t.Fatalf("broken file handling: %+v", res)
	}
}

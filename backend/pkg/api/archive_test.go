package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func tarGzOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var tb bytes.Buffer
	tw := tar.NewWriter(&tb)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	var gb bytes.Buffer
	gw := gzip.NewWriter(&gb)
	gw.Write(tb.Bytes())
	gw.Close()
	return gb.Bytes()
}

func gzOf(t *testing.T, body string) []byte {
	t.Helper()
	var gb bytes.Buffer
	gw := gzip.NewWriter(&gb)
	gw.Write([]byte(body))
	gw.Close()
	return gb.Bytes()
}

func TestArchiveKind(t *testing.T) {
	for want, raw := range map[string][]byte{
		"zip": zipOf(t, map[string]string{"a.csv": "x"}), "gzip": gzOf(t, "x"), "bzip2": []byte("BZh91AY"),
		"7z": []byte("7z\xbc\xaf\x27\x1c\x00"), "rar": []byte("Rar!\x1a\x07\x00"), "": []byte("Виписка по картці"),
	} {
		if got := archiveKind(raw); got != want {
			t.Errorf("%q: got %q", want, got)
		}
	}
	if got := archiveKind(tarGzOf(t, map[string]string{"a.csv": "x"})); got != "gzip" {
		t.Errorf("tar.gz снаружи — gzip, получили %q", got)
	}
}

func TestArchiveImport(t *testing.T) {
	t.Setenv("ECONOMIC_SOURCES_DIR", t.TempDir())
	e := newEnv(t)
	post := func(name string, raw []byte, path string, h func(http.ResponseWriter, *http.Request)) (int, map[string]any, string) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write(raw)
		mw.Close()
		req := httptest.NewRequest(http.MethodPost, path, &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		h(rec, req)
		var m map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &m)
		return rec.Code, m, rec.Body.String()
	}
	imp := func(name string, raw []byte) (int, map[string]any, string) {
		return post(name, raw, "/api/sync/import-csv", e.s.HandleImportCSV)
	}
	newer := csvCard1 + "13.03.2025 08:00:00;4149****7777;;Ще кава;5814;-10,00;0,00\n"

	// до импорта: проверка архива с одной выпиской и служебным файлом
	z := zipOf(t, map[string]string{"Петро Іваненко.csv": csvCard1, "README.txt": "метаданные", "__MACOSX/._x.csv": "мусор"})
	if c, m, b := post("export.zip", z, "/api/sources/inspect", e.s.HandleInspectSource); c != 200 || m["files"] != float64(1) || m["count"] == float64(0) {
		t.Fatalf("inspect: %d %s", c, b)
	}
	if c, m, b := imp("export.zip", z); c != 200 || m["files"] != float64(1) || m["imported_count"] == float64(0) {
		t.Fatalf("zip с одним файлом: %d %s", c, b)
	}
	// тот же архив снова: все файлы уже загружены
	if c, _, b := imp("export.zip", z); c != 409 || !strings.Contains(b, "уже загружены") {
		t.Fatalf("повторный архив: %d %s", c, b)
	}
	// архив из старого и дополненного файла одного счёта: дубль пропускается, новое добавляется
	z2 := zipOf(t, map[string]string{"a.csv": csvCard1, "b.csv": newer})
	if c, m, b := imp("two.zip", z2); c != 200 || m["files"] != float64(1) || m["skipped"] != float64(1) {
		t.Fatalf("дубль + новое: %d %s", c, b)
	}

	// gzip с одним файлом и tar.gz
	if c, _, b := imp("card2.csv.gz", gzOf(t, csvCard2)); c != 200 {
		t.Fatalf("gz: %d %s", c, b)
	}
	t.Setenv("ECONOMIC_SOURCES_DIR", t.TempDir()) // чистое хранилище источников для второй среды
	e2 := newEnv(t)
	imp2 := func(name string, raw []byte) (int, string) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write(raw)
		mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/sync/import-csv", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		e2.s.HandleImportCSV(rec, req)
		return rec.Code, rec.Body.String()
	}
	// разные источники в одном архиве — отказ без записи
	mixed := zipOf(t, map[string]string{"a.csv": csvCard1, "b.csv": csvCard2})
	if c, b := imp2("mixed.zip", mixed); c != 400 || !strings.Contains(b, "разным источникам") {
		t.Fatalf("смешанный архив: %d %s", c, b)
	}
	if accs, _ := e2.s.storage.GetAccounts(); len(accs) != 0 {
		t.Fatalf("после отказа в базе ничего нет: %+v", accs)
	}
	if c, b := imp2("x.tgz", tarGzOf(t, map[string]string{"dir/one.csv": csvCard1})); c != 200 {
		t.Fatalf("tar.gz: %d %s", c, b)
	}

	// без выписок внутри, 7z/rar, битый zip
	if c, _, b := imp("notes.zip", zipOf(t, map[string]string{"README.txt": "x"})); c != 400 || !strings.Contains(b, "нет файлов выписок") {
		t.Fatalf("пустой архив: %d %s", c, b)
	}
	if c, _, b := imp("a.7z", []byte("7z\xbc\xaf\x27\x1c\x00\x04")); c != 400 || !strings.Contains(b, "не поддерживается") {
		t.Fatalf("7z: %d %s", c, b)
	}
	if c, _, _ := imp("bad.zip", []byte("PK\x03\x04мусор")); c != 400 {
		t.Fatalf("битый zip: %d", c)
	}
}

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

	"economic/pkg/db"
	"economic/pkg/source"
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
		m, _ := filepath.Glob(filepath.Join(dir, pan, "*"+source.Ext))
		if len(m) != 1 {
			t.Fatalf("folder %s: %v", pan, m)
		}
		if fi, _ := os.Stat(m[0]); fi.Mode().Perm() != 0o600 {
			t.Fatalf("source must be owner-only: %v", fi.Mode())
		}
		f, err := source.Read(m[0])
		if err != nil || f.Name != "Петро Іваненко.csv" || f.Created == 0 || f.Count() == 0 {
			t.Fatalf("name/created lost: %+v %v", f, err)
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
	os.WriteFile(filepath.Join(dir, "4149****7777", "2026-01-01_deadbeef.zoe"), []byte("мусор"), 0o600)
	rec = httptest.NewRecorder()
	e.s.HandleLoadSources(rec, httptest.NewRequest(http.MethodPost, "/api/sources/load", nil))
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Total != 3 || res.Loaded != 2 || len(res.Failed) != 1 {
		t.Fatalf("broken file handling: %+v", res)
	}
}

// Старый кэш (сырые файлы <хэш>_<имя>) сам переезжает в .zoe, оригиналы — в _legacy.
func TestLegacySourcesMigrate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECONOMIC_SOURCES_DIR", dir)
	e := newEnv(t)
	old := filepath.Join(dir, "4149****7777")
	os.MkdirAll(old, 0o700)
	os.WriteFile(filepath.Join(old, "0123456789abcdef0123_Петро Іваненко.csv"), []byte(csvCard1), 0o600)
	os.WriteFile(filepath.Join(old, "0123456789abcdef0124_мусор.csv"), []byte("не выписка"), 0o600)

	rec := httptest.NewRecorder()
	e.s.HandleLoadSources(rec, httptest.NewRequest(http.MethodPost, "/api/sources/load", nil))
	var res struct {
		Total, Loaded, Transactions int
		Failed                      []loadFailure
	}
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Loaded != 1 || res.Transactions != 2 || len(res.Failed) != 1 {
		t.Fatalf("%+v", res)
	}
	if m, _ := filepath.Glob(filepath.Join(old, "*"+source.Ext)); len(m) != 1 {
		t.Fatalf("zoe not created: %v", m)
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "_legacy", "4149****7777", "*")); len(m) != 1 {
		t.Fatalf("original not moved: %v", m)
	}
	if _, err := os.Stat(filepath.Join(old, "0123456789abcdef0124_мусор.csv")); err != nil {
		t.Fatal("unparsable file must stay in place")
	}
}

// Выборочные «Сбросить» и «Загрузить»: пусто — все счета, иначе только перечисленные.
func TestSourcesSelectiveResetLoad(t *testing.T) {
	t.Setenv("ECONOMIC_SOURCES_DIR", t.TempDir())
	e := newEnv(t)
	r1 := upload(t, e.s, "Петро Іваненко.csv", csvCard1)
	r2 := upload(t, e.s, "Іван.csv", csvCard2)
	id1, id2 := r1["account_id"].(string), r2["account_id"].(string)

	call := func(h http.HandlerFunc, ids ...string) {
		body, _ := json.Marshal(map[string]any{"accounts": ids})
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(body)))
		if rec.Code != 200 {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	}
	inv := func() map[string]invAccount {
		rec := httptest.NewRecorder()
		e.s.HandleSources(rec, httptest.NewRequest(http.MethodGet, "/api/sources", nil))
		var out struct{ Accounts []invAccount }
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		m := map[string]invAccount{}
		for _, a := range out.Accounts {
			m[a.ID] = a
		}
		return m
	}

	m := inv()
	if !m[id1].Loaded || !m[id2].Loaded || len(m[id1].Files) != 1 || m[id1].Records != 2 || len(m[id1].Cells) != invCells {
		t.Fatalf("inventory: %+v", m)
	}
	if !strings.Contains(m[id1].Cells, "1") || !strings.Contains(m[id1].Cells, "0") {
		t.Fatalf("timeline must show data and the gap up to now: %s", m[id1].Cells)
	}

	call(e.s.HandleResetSources, id1) // 10 → 00
	m = inv()
	if m[id1].Loaded || !m[id2].Loaded {
		t.Fatalf("selective reset: %+v", m)
	}
	call(e.s.HandleLoadSources, id1) // 00 → 10
	if m = inv(); !m[id1].Loaded || !m[id2].Loaded {
		t.Fatalf("selective load: %+v", m)
	}
	call(e.s.HandleResetSources) // пусто = все
	if m = inv(); m[id1].Loaded || m[id2].Loaded || len(m[id1].Files) != 1 {
		t.Fatalf("global reset must keep sources: %+v", m)
	}
	call(e.s.HandleLoadSources)
	if m = inv(); !m[id1].Loaded || !m[id2].Loaded {
		t.Fatalf("global load: %+v", m)
	}
}

func TestCoverCells(t *testing.T) {
	// ось 0…960, 96 клеток по 10 с; снято 100…500, операции только в 130 и 450
	got := coverCells([]db.Span{{From: 100, To: 500}}, []int64{130, 450}, 0, 960)
	for i, want := range map[int]byte{5: '0', 10: '2', 13: '1', 20: '2', 45: '1', 60: '0'} {
		if got[i] != want {
			t.Errorf("cell %d: got %c want %c (%s)", i, got[i], want, got)
		}
	}
}

// Новые правила импорта: «Добавить» — только новый счёт, «Обновить» — только этот счёт, алиас существующего счёта не затирается.
func TestImportRules(t *testing.T) {
	t.Setenv("ECONOMIC_SOURCES_DIR", t.TempDir())
	e := newEnv(t)

	send := func(name, content string, form map[string]string) (int, string) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write([]byte(content))
		for k, v := range form {
			mw.WriteField(k, v)
		}
		mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/sync/import-csv", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		e.s.HandleImportCSV(rec, req)
		return rec.Code, rec.Body.String()
	}
	inspect := func(name, content, account string) inspection {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write([]byte(content))
		if account != "" {
			mw.WriteField("account", account)
		}
		mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/sources/inspect", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		e.s.HandleInspectSource(rec, req)
		var in inspection
		if err := json.Unmarshal(rec.Body.Bytes(), &in); err != nil || rec.Code != 200 {
			t.Fatalf("inspect: %d %s", rec.Code, rec.Body.String())
		}
		return in
	}
	codes := func(in inspection) string {
		var c []string
		for _, p := range in.Problems {
			c = append(c, p.Code)
		}
		return strings.Join(c, ",")
	}

	// новый счёт через «Добавить»
	if in := inspect("Петро Іваненко.csv", csvCard1, ""); in.Exists || len(in.Problems) != 0 {
		t.Fatalf("новый счёт без проблем: %+v", in)
	}
	if c, b := send("Петро Іваненко.csv", csvCard1, map[string]string{"mode": "add"}); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	accID := "raif_UA583003350000000026001234567"

	// тот же файл снова — дубль
	if in := inspect("copy.csv", csvCard1, ""); !strings.Contains(codes(in), "duplicate") {
		t.Fatalf("дубль: %+v", in)
	}
	// «Добавить» нашло существующий счёт: блок + выбор «обновить»; другое имя — конфликт алиаса
	newer := strings.Replace(csvCard1, "12:00:00", "12:00:00", 1) + "13.03.2025 08:00:00;4149****7777;;Ще кава;5814;-10,00;0,00\n"
	in := inspect("Ivanenko Petro.csv", newer, "")
	if !in.Exists || !strings.Contains(codes(in), "exists") || !strings.Contains(codes(in), "alias") || in.Overlap == nil || in.Overlap.Known != 2 || in.Overlap.New != 1 {
		t.Fatalf("существующий счёт: %+v overlap=%+v", in, in.Overlap)
	}
	if c, _ := send("Ivanenko Petro.csv", newer, map[string]string{"mode": "add"}); c != 409 {
		t.Fatalf("«Добавить» не должно молча обновлять существующий счёт: %d", c)
	}
	// «Обновить» с подтверждением и оставленным алиасом
	if c, b := send("Ivanenko Petro.csv", newer, map[string]string{"mode": "update", "account": accID, "alias": "keep"}); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
	accs, _ := e.st.GetAccounts()
	if len(accs) != 1 || !strings.HasPrefix(accs[0].Name, "Петро Іваненко.csv") {
		t.Fatalf("алиас должен остаться прежним: %+v", accs)
	}
	if txs, _ := e.st.GetTransactions(10); len(txs) != 3 {
		t.Fatalf("после обновления 3 операции: %d", len(txs))
	}
	// «Обновить» файлом другого счёта — отказ
	if c, _ := send("Іван.csv", csvCard2, map[string]string{"mode": "update", "account": accID}); c != 409 {
		t.Fatalf("файл другого счёта должен отклоняться: %d", c)
	}
	if in := inspect("Іван.csv", csvCard2, accID); !strings.Contains(codes(in), "other_account") {
		t.Fatalf("%+v", in)
	}
	// тот же счёт, но общих операций нет — предупреждение, записывается только с force
	other := strings.Replace(newer, "Кава", "Зовсім інше", 1)
	other = strings.Replace(other, "Ще кава", "І ще інше", 1)
	other = strings.Replace(other, "Переказ з картки: Ivanenko Petro", "Інший переказ", 1)
	if in := inspect("Петро Іваненко.csv", other, accID); !strings.Contains(codes(in), "no_match") {
		t.Fatalf("несовпадение операций: %+v overlap=%+v", in, in.Overlap)
	}
	if c, _ := send("Петро Іваненко.csv", other, map[string]string{"mode": "update", "account": accID}); c != 409 {
		t.Fatalf("без force: %d", c)
	}
	if c, b := send("Петро Іваненко.csv", other, map[string]string{"mode": "update", "account": accID, "force": "1"}); c != 200 {
		t.Fatalf("%d %s", c, b)
	}
}

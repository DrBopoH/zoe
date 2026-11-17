package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"economic/pkg/db"
	"economic/pkg/icu"
	"economic/pkg/pdftext"
)

func call(t *testing.T, h http.HandlerFunc, method, url, body string) (int, []byte) {
	t.Helper()
	var rd *bytes.Buffer
	if body != "" {
		rd = bytes.NewBufferString(body)
	} else {
		rd = &bytes.Buffer{}
	}
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(method, url, rd))
	return w.Code, w.Body.Bytes()
}

func newICUServer(t *testing.T) *Server {
	t.Helper()
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return NewServer(st, nil, nil)
}

// разобранный обезличенный отчёт (фикстура pkg/icu) — без pdftotext, чтобы тест не зависел от poppler
func fixtureReport(t *testing.T) *icu.Report {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "icu", "testdata", "report_bbox.xml"))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := icu.ParseWords(pdftext.ParseBBox(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestHTTPICUReportBecomesAnOrdinaryAccount(t *testing.T) {
	s := newICUServer(t)
	res, err := s.saveICUReport("ОВГЗ Тест.pdf", fixtureReport(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.AccountID != "icu_2610210000" || res.Count != 6 || res.Card != "icu" {
		t.Fatalf("результат импорта: %+v", res)
	}
	// тот же путь, что у всех счетов: /api/accounts
	c, b := call(t, s.HandleGetAccounts, "GET", "/api/accounts", "")
	var accs []db.Account
	json.Unmarshal(b, &accs)
	if c != 200 || len(accs) != 1 || accs[0].BankKey != "icu" || accs[0].Balance != 5152.14 {
		t.Fatalf("accounts: %d %s", c, b)
	}
	c, b = call(t, s.HandleICULedger, "GET", "/api/icu/ledger?account_id="+res.AccountID, "")
	var ov db.ICUOverview
	json.Unmarshal(b, &ov)
	if c != 200 || len(ov.Rows) != 6 || len(ov.Positions) != 2 {
		t.Fatalf("ledger: %d %s", c, b)
	}
	// редактировать нечего: журнал только из отчёта
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		if c, _ := call(t, s.HandleICULedger, m, "/api/icu/ledger?account_id="+res.AccountID, `{}`); c != http.StatusMethodNotAllowed {
			t.Fatalf("%s по журналу: %d", m, c)
		}
	}
	// повторная загрузка (кнопка «Загрузить») не плодит дублей
	if _, err := s.saveICUReport("ОВГЗ Тест.pdf", fixtureReport(t)); err != nil {
		t.Fatal(err)
	}
	_, b = call(t, s.HandleICULedger, "GET", "/api/icu/ledger?account_id="+res.AccountID, "")
	json.Unmarshal(b, &ov)
	if len(ov.Rows) != 6 {
		t.Fatalf("дубли после повторной загрузки: %d", len(ov.Rows))
	}
}

func TestICUReportSourceFolder(t *testing.T) {
	if got := maskPAN("icu"); got != "icu" {
		t.Fatalf("папка источника: %q", got)
	}
}

func TestNotAnICUReportIsNotTakenForOne(t *testing.T) {
	if icu.IsReport([]byte("a,b,c\n1,2,3")) {
		t.Fatal("csv — не отчёт ICU")
	}
}

func TestHTTPBondReference(t *testing.T) {
	s := newICUServer(t)
	bond := `{"isin":"ua4000239081","name":"ОВДП","maturity":"2030-02-20","coupon_rate":16.44}`
	if c, b := call(t, s.HandleICUBonds, "POST", "/api/icu/bonds", bond); c != 200 {
		t.Fatalf("post: %d %s", c, b)
	}
	if c, _ := call(t, s.HandleICUBonds, "POST", "/api/icu/bonds", `{"isin":"X","maturity":"завтра","coupon_rate":1}`); c != 400 {
		t.Fatalf("bad maturity: %d", c)
	}
	c, b := call(t, s.HandleICUBonds, "GET", "/api/icu/bonds", "")
	var list []db.Bond
	json.Unmarshal(b, &list)
	if c != 200 || len(list) != 1 || list[0].ISIN != "UA4000239081" || list[0].Nominal != 1000 {
		t.Fatalf("get: %d %s", c, b)
	}
	// облигация с записями в журнале удаляется: журнал цел, позиция остаётся без условий
	if _, err := s.saveICUReport("ОВГЗ Тест.pdf", fixtureReport(t)); err != nil {
		t.Fatal(err)
	}
	if c, _ := call(t, s.HandleICUBonds, "DELETE", "/api/icu/bonds?isin=ua4000239081", ""); c != 200 {
		t.Fatalf("delete used bond: %d", c)
	}
	if c, _ := call(t, s.HandleICUBonds, "POST", "/api/icu/bonds", `{"isin":"UA0000000001","maturity":"2030-02-20","coupon_rate":1}`); c != 200 {
		t.Fatalf("post free bond: %d", c)
	}
	if c, _ := call(t, s.HandleICUBonds, "DELETE", "/api/icu/bonds?isin=UA0000000001", ""); c != 200 {
		t.Fatalf("delete free bond: %d", c)
	}
}

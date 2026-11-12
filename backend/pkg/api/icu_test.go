package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"economic/pkg/db"
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

func TestHTTPManualAccountAndLedger(t *testing.T) {
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)

	// счёт: без имени и с плохим IBAN — 400
	if c, _ := call(t, s.HandleManualAccounts, "POST", "/api/manual-accounts", `{"name":" "}`); c != 400 {
		t.Fatalf("empty name: %d", c)
	}
	if c, _ := call(t, s.HandleManualAccounts, "POST", "/api/manual-accounts", `{"name":"ICU","iban":"xx"}`); c != 400 {
		t.Fatalf("bad iban: %d", c)
	}
	c, b := call(t, s.HandleManualAccounts, "POST", "/api/manual-accounts", `{"name":"ICU Trader","iban":"UA503220010000026009000000001"}`)
	if c != 200 {
		t.Fatalf("create: %d %s", c, b)
	}
	var acc db.ManualAccount
	json.Unmarshal(b, &acc)
	if acc.ID == "" || acc.BankKey != "icu" {
		t.Fatalf("account: %+v", acc)
	}

	// запись журнала: дата строкой, вывод положительным числом
	c, b = call(t, s.HandleICULedger, "POST", "/api/icu/ledger", `{"account_id":"`+acc.ID+`","date":"2026-10-23","kind":"deposit","amount":2048.58}`)
	if c != 200 {
		t.Fatalf("ledger add: %d %s", c, b)
	}
	if c, _ := call(t, s.HandleICULedger, "POST", "/api/icu/ledger", `{"account_id":"`+acc.ID+`","date":"вчера","kind":"deposit","amount":1}`); c != 400 {
		t.Fatalf("bad date: %d", c)
	}
	call(t, s.HandleICULedger, "POST", "/api/icu/ledger", `{"account_id":"`+acc.ID+`","date":"2026-10-24","kind":"withdraw","amount":48.58}`)

	// счета и операции отдают ручной счёт и записи журнала
	c, b = call(t, s.HandleGetAccounts, "GET", "/api/accounts", "")
	var accs []db.Account
	json.Unmarshal(b, &accs)
	if c != 200 || len(accs) != 1 || !accs[0].Manual || accs[0].Balance != 2000 {
		t.Fatalf("accounts: %d %+v", c, accs)
	}
	c, b = call(t, s.HandleGetTransactions, "GET", "/api/transactions", "")
	var txs []db.Transaction
	json.Unmarshal(b, &txs)
	if c != 200 || len(txs) != 2 || !txs[0].Internal || txs[0].AccountID != acc.ID {
		t.Fatalf("transactions: %d %+v", c, txs)
	}

	// журнал с цветом сверки: кеша нет → обе записи оранжевые
	c, b = call(t, s.HandleICULedger, "GET", "/api/icu/ledger?account_id="+acc.ID, "")
	var ov db.ICUOverview
	json.Unmarshal(b, &ov)
	if c != 200 || len(ov.Rows) != 2 || ov.Rows[0].Status != db.StatusOrange || ov.Balance != 2000 {
		t.Fatalf("overview: %d %+v", c, ov)
	}
	if c, _ := call(t, s.HandleICULedger, "GET", "/api/icu/ledger?account_id=nope", ""); c != 400 {
		t.Fatalf("unknown account: %d", c)
	}

	// accept без счёта — 400, с пустым кешем принято 0
	if c, _ := call(t, s.HandleICUAccept, "POST", "/api/icu/accept", `{}`); c != 400 {
		t.Fatalf("accept empty: %d", c)
	}
	c, b = call(t, s.HandleICUAccept, "POST", "/api/icu/accept", `{"account_id":"`+acc.ID+`","all":true}`)
	if c != 200 || string(bytes.TrimSpace(b)) != `{"accepted":0}` {
		t.Fatalf("accept: %d %s", c, b)
	}

	// удаление записи и счёта
	id := ov.Rows[0].ID
	if c, _ := call(t, s.HandleICULedger, "DELETE", "/api/icu/ledger?id="+itoa(id), ""); c != 200 {
		t.Fatalf("delete row: %d", c)
	}
	if c, _ := call(t, s.HandleManualAccounts, "DELETE", "/api/manual-accounts?id="+acc.ID, ""); c != 200 {
		t.Fatalf("delete account: %d", c)
	}
	_, b = call(t, s.HandleGetAccounts, "GET", "/api/accounts", "")
	if string(bytes.TrimSpace(b)) != "[]" && string(bytes.TrimSpace(b)) != "null" {
		t.Fatalf("accounts after delete: %s", b)
	}
}

func itoa(v int64) string { return string(bytes.TrimSpace([]byte(jsonNum(v)))) }
func jsonNum(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestHTTPBonds(t *testing.T) {
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)
	_, b := call(t, s.HandleManualAccounts, "POST", "/api/manual-accounts", `{"name":"ICU"}`)
	var acc db.ManualAccount
	json.Unmarshal(b, &acc)

	bond := `{"account_id":"` + acc.ID + `","isin":"ua4000239081","name":"ОВДП","maturity":"2030-02-20","coupon_rate":16.44}`
	if c, b := call(t, s.HandleICUBonds, "POST", "/api/icu/bonds", bond); c != 200 {
		t.Fatalf("bond: %d %s", c, b)
	}
	if c, _ := call(t, s.HandleICUBonds, "POST", "/api/icu/bonds", `{"account_id":"`+acc.ID+`","isin":"X","maturity":"завтра","coupon_rate":1}`); c != 400 {
		t.Fatalf("bad maturity: %d", c)
	}
	if c, _ := call(t, s.HandleICULedger, "POST", "/api/icu/ledger", `{"account_id":"`+acc.ID+`","date":"2026-11-05","kind":"buy","amount":3070}`); c != 400 {
		t.Fatalf("buy without isin: %d", c)
	}
	c, b := call(t, s.HandleICULedger, "POST", "/api/icu/ledger", `{"account_id":"`+acc.ID+`","date":"2026-11-05","kind":"buy","amount":3070,"isin":"UA4000239081","qty":3}`)
	if c != 200 {
		t.Fatalf("buy: %d %s", c, b)
	}
	c, b = call(t, s.HandleICULedger, "GET", "/api/icu/ledger?account_id="+acc.ID, "")
	var ov db.ICUOverview
	json.Unmarshal(b, &ov)
	if c != 200 || len(ov.Positions) != 1 || len(ov.Events) == 0 || len(ov.Bonds) != 1 {
		t.Fatalf("overview: %d %s", c, b)
	}
	if c, _ := call(t, s.HandleICUBonds, "DELETE", "/api/icu/bonds?account_id="+acc.ID+"&isin=UA4000239081", ""); c != 400 {
		t.Fatalf("delete used bond: %d", c)
	}
}

func TestHTTPBondsByIBAN(t *testing.T) {
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := NewServer(st, nil, nil)
	const iban = "UA673809460000026009000000777"
	body := `{"iban":"` + iban + `","isin":"UA4000239081","maturity":"2030-02-20","coupon_rate":16.44}`
	if c, b := call(t, s.HandleICUBonds, "POST", "/api/icu/bonds", body); c != 200 {
		t.Fatalf("post: %d %s", c, b)
	}
	c, b := call(t, s.HandleICUBonds, "GET", "/api/icu/bonds?iban="+iban, "")
	var list []db.Bond
	json.Unmarshal(b, &list)
	if c != 200 || len(list) != 1 || list[0].Nominal != 1000 {
		t.Fatalf("get: %d %s", c, b)
	}
	if c, _ := call(t, s.HandleICUBonds, "POST", "/api/icu/bonds", `{"iban":"UA583003350000000026001234567","isin":"X","maturity":"2030-02-20","coupon_rate":1}`); c != 400 {
		t.Fatalf("чужой IBAN: %d", c)
	}
	if c, _ := call(t, s.HandleICUBonds, "DELETE", "/api/icu/bonds?iban="+iban+"&isin=ua4000239081", ""); c != 200 {
		t.Fatalf("delete: %d", c)
	}
}

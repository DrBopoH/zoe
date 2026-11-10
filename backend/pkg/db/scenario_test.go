package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"economic/pkg/mono"
)

const (
	ibMono = "UA213220010000000000000006001"
	ibRaif = "UA583003350000000026001234567"
	ibPriv = "UA303052990000026205123456789"
)

func newS(t *testing.T) (*Storage, string, string) {
	d := t.TempDir()
	c, k := filepath.Join(d, "cache.db"), filepath.Join(d, "core.db")
	s, err := NewStorage(c, k)
	if err != nil {
		t.Fatal(err)
	}
	return s, c, k
}

func fillCache(t *testing.T, s *Storage) {
	// банк-API моно: чёрная карта с ПЕРЕВЫПУСКОМ (2 PAN на 1 IBAN) + ФОП
	if err := s.SaveAccounts([]mono.AccountDTO{
		{ID: "m1", IBAN: ibMono, Type: "black", MaskedPan: []string{"537541******1111", "444111******2222"}, Balance: 1000},
		{ID: "m2", IBAN: "UA00322001000002600700000001", Type: "fop"},
	}, "Іваненко Петро"); err != nil {
		t.Fatal(err)
	}
	// файл райфа
	if err := s.SaveImportedAccount(mono.AccountDTO{ID: "raif_" + ibRaif, IBAN: ibRaif, Type: "raif", MaskedPan: []string{"4149****7777"}}, "Петро Іваненко.csv"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTransactions("raif_"+ibRaif, []mono.StatementItemDTO{{ID: "r1", Time: 100, Amount: 5000, Description: "x"}}); err != nil {
		t.Fatal(err)
	}
	// моно: контакт «drbopoh raif» ведёт на айбан райфа
	if err := s.SaveTransactions("m1", []mono.StatementItemDTO{
		{ID: "t1", Time: 200, Amount: -1000, CounterName: "drbopoh raif", CounterIban: ibRaif},
		{ID: "t2", Time: 300, Amount: -500, Description: "Від: Ivanenko Petro", CounterIban: ibPriv},
	}); err != nil {
		t.Fatal(err)
	}
}

func user(t *testing.T, s *Storage) User {
	us, err := s.GetUsers()
	if err != nil || len(us) != 1 {
		t.Fatalf("users %v %v", us, err)
	}
	return us[0]
}

func links(u User, alias string) map[string]AliasLink {
	m := map[string]AliasLink{}
	for _, l := range u.AliasLinks {
		if aliasKey(l.Alias) == aliasKey(alias) {
			m[idKey(l.AccountIdentifier)] = l
		}
	}
	return m
}

func TestFlow(t *testing.T) {
	s, _, _ := newS(t)
	fillCache(t, s)
	if err := s.CreateUser(User{ID: "u1", Name: "Док"}); err != nil {
		t.Fatal(err)
	}

	// До привязки статика пуста — кеш ничего сам не записал.
	var n int
	s.coreDB.QueryRow(`SELECT COUNT(*) FROM alias_accounts`).Scan(&n)
	if n != 0 {
		t.Fatalf("static must be empty before attach, got %d", n)
	}

	// Привязка алиасов
	for _, a := range []string{"Іваненко Петро", "Петро Іваненко.csv", "drbopoh raif"} {
		if err := s.AddUserAlias("u1", a); err != nil {
			t.Fatal(err)
		}
	}
	u := user(t, s)
	m := links(u, "Іваненко Петро")
	if len(m) != 5 { // 2 IBAN + 2 PAN (black) ... fop без pan: ibMono, fop iban, 2 pans = 4
		t.Logf("mono links: %d", len(m))
	}
	if l, ok := m[idKey(ibMono)]; !ok || !l.Confirmed {
		t.Fatalf("mono IBAN missing/unconfirmed: %+v", m)
	}
	// оба PAN под одним IBAN
	for _, p := range []string{"537541******1111", "444111******2222"} {
		l, ok := m[idKey(p)]
		if !ok || l.ParentIdentifier != ibMono {
			t.Fatalf("pan %s: %+v", p, l)
		}
	}
	// drbopoh raif ведёт на raif IBAN и его PAN, но НЕ на моно
	d := links(u, "drbopoh raif")
	if _, ok := d[idKey(ibRaif)]; !ok {
		t.Fatalf("drbopoh must link raif iban: %+v", d)
	}
	if _, ok := d[idKey(ibMono)]; ok {
		t.Fatal("drbopoh must not link mono")
	}
	// Резолв операций
	txs, _ := s.GetTransactions(10)
	for _, tx := range txs {
		if tx.ID == "t1" && tx.ResolvedUserID != "u1" {
			t.Fatalf("t1 not resolved: %+v", tx)
		}
		if tx.ID == "t2" && tx.ResolvedUserID != "" {
			t.Fatalf("t2 must be unresolved: %+v", tx)
		}
	}
	// кеш не содержит данных лица
	if hasColumn(s.cacheDB, "transactions", "resolved_user_id") {
		t.Fatal("cache must not store resolved_user_id")
	}

	// Снос кеша → структура помнится, но жёлтая
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	u = user(t, s)
	for _, l := range u.AliasLinks {
		if l.Confirmed {
			t.Fatalf("after wipe all must be unconfirmed: %+v", l)
		}
	}
	if len(links(u, "Іваненко Петро")) == 0 {
		t.Fatal("structure lost after wipe")
	}

	// Перенакат кеша → снова зелёные
	fillCache(t, s)
	u = user(t, s)
	if !links(u, "Іваненко Петро")[idKey(ibMono)].Confirmed {
		t.Fatal("not re-confirmed")
	}

	// Снос структуры при наличии кеша → теневой бан
	if err := s.UnlinkAccount("Іваненко Петро", "444111******2222"); err != nil {
		t.Fatal(err)
	}
	u = user(t, s)
	if _, ok := links(u, "Іваненко Петро")[idKey("444111******2222")]; ok {
		t.Fatal("pan not removed")
	}
	if len(u.AliasBans) != 1 {
		t.Fatalf("expected 1 ban: %+v", u.AliasBans)
	}
	// перенакат и новый sync НЕ воскрешают
	fillCache(t, s)
	s.syncAliases(nil)
	if _, ok := links(user(t, s), "Іваненко Петро")[idKey("444111******2222")]; ok {
		t.Fatal("banned pan resurrected")
	}
	// у другого алиаса (drbopoh) тот же счёт не затронут
	if _, ok := links(user(t, s), "drbopoh raif")[idKey(ibRaif)]; !ok {
		t.Fatal("other alias affected by ban")
	}

	// Ручное добавление в модалке снимает бан
	if err := s.LinkAccount("u1", "Іваненко Петро", "444111******2222", ""); err != nil {
		t.Fatal(err)
	}
	u = user(t, s)
	if len(u.AliasBans) != 0 {
		t.Fatal("ban not lifted")
	}
	l := links(u, "Іваненко Петро")[idKey("444111******2222")]
	if l.ParentIdentifier != ibMono || !l.Confirmed {
		t.Fatalf("manual link should resolve parent from cache: %+v", l)
	}

	// Снова под бан, затем ОТВЯЗКА алиаса от лица → бан гаснет, структура помнится
	_ = s.UnlinkAccount("Іваненко Петро", "444111******2222")
	if err := s.RemoveUserAlias("Іваненко Петро"); err != nil {
		t.Fatal(err)
	}
	u = user(t, s)
	if len(u.AliasBans) != 0 || len(links(u, "Іваненко Петро")) != 0 {
		t.Fatal("detached alias must vanish from the person")
	}
	st, _ := s.GetAliasStructures()
	found := false
	for _, a := range st {
		if aliasKey(a.Alias) == aliasKey("Іваненко Петро") && a.UserID == "" && len(a.Accounts) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("orphan alias must remember structure: %+v", st)
	}
	// возврат к лицу → кеш снова пишет полную структуру (бана нет)
	if err := s.AddUserAlias("u1", "Іваненко Петро"); err != nil {
		t.Fatal(err)
	}
	if _, ok := links(user(t, s), "Іваненко Петро")[idKey("444111******2222")]; !ok {
		t.Fatal("pan must come back after re-attach")
	}

	// Карта известна глобально (iban_pans), поэтому ручной снос держится даже без кеша
	// и не воскресает после перенакатки; ручное добавление возвращает её.
	_ = s.WipeCache()
	if err := s.UnlinkAccount("Іваненко Петро", "444111******2222"); err != nil {
		t.Fatal(err)
	}
	u = user(t, s)
	if len(u.AliasBans) == 0 {
		t.Fatal("explicit unlink of a globally-known card must stick")
	}
	fillCache(t, s)
	if _, ok := links(user(t, s), "Іваненко Петро")[idKey("444111******2222")]; ok {
		t.Fatal("banned card resurrected by cache")
	}
	if err := s.LinkAccount("u1", "Іваненко Петро", "444111******2222", ibMono); err != nil {
		t.Fatal(err)
	}
	if _, ok := links(user(t, s), "Іваненко Петро")[idKey("444111******2222")]; !ok {
		t.Fatal("manual relink must restore the card")
	}

	// Снос IBAN каскадом уносит его PAN
	if err := s.UnlinkAccount("Іваненко Петро", ibMono); err != nil {
		t.Fatal(err)
	}
	for k, l := range links(user(t, s), "Іваненко Петро") {
		if idKey(l.ParentIdentifier) == idKey(ibMono) || k == idKey(ibMono) {
			t.Fatalf("cascade failed: %+v", l)
		}
	}
	// Бан IBAN держит и новые PAN под ним (перевыпуск)
	s.SaveAccounts([]mono.AccountDTO{{ID: "m1", IBAN: ibMono, Type: "black", MaskedPan: []string{"999999******3333"}}}, "Іваненко Петро")
	if _, ok := links(user(t, s), "Іваненко Петро")[idKey("999999******3333")]; ok {
		t.Fatal("new pan under banned iban must stay banned")
	}
}

func TestPanDedupAndMain(t *testing.T) {
	s, _, _ := newS(t)
	s.SaveAccounts([]mono.AccountDTO{{ID: "a", IBAN: ibMono, Type: "black", MaskedPan: []string{"4874****8644"}}}, "X")
	s.SaveAccounts([]mono.AccountDTO{{ID: "a", IBAN: ibMono, Type: "black", MaskedPan: []string{"487407******8644", "111122******9999"}}}, "X")
	// файл с другим PAN главную карту не меняет
	s.SaveImportedAccount(mono.AccountDTO{ID: "a", IBAN: ibMono, Type: "black", MaskedPan: []string{"555555******0000"}}, "X")
	as, _ := s.GetAccounts()
	a := as[0]
	if len(a.Pans) != 3 || a.MaskedPan != "487407******8644" {
		t.Fatalf("pans %v main %s", a.Pans, a.MaskedPan)
	}
}

func TestOverridesSurviveWipe(t *testing.T) {
	s, _, _ := newS(t)
	s.SaveAccounts([]mono.AccountDTO{{ID: "a", IBAN: ibMono, Type: "black"}}, "X")
	s.UpdateCreditLimit("a", 5000)
	_ = s.WipeCache()
	s.SaveAccounts([]mono.AccountDTO{{ID: "a", IBAN: ibMono, Type: "black"}}, "X")
	as, _ := s.GetAccounts()
	if as[0].CreditLimit != 5000 {
		t.Fatalf("credit limit lost: %v", as[0].CreditLimit)
	}
}

func TestBalanceApiVsImport(t *testing.T) {
	s, _, _ := newS(t)
	s.SaveAccounts([]mono.AccountDTO{{ID: "m", IBAN: ibMono, Type: "black", Balance: 123456}}, "X")
	s.SaveTransactions("m", []mono.StatementItemDTO{{ID: "1", Amount: 100}})
	s.SaveAccounts([]mono.AccountDTO{{ID: "m", IBAN: ibMono, Type: "black", Balance: 200000}}, "X")
	s.SaveImportedAccount(mono.AccountDTO{ID: "privat_x", IBAN: ibPriv, Type: "privat"}, "f.pdf")
	s.SaveTransactions("privat_x", []mono.StatementItemDTO{{ID: "p1", Amount: 1000}, {ID: "p2", Amount: -250}})
	as, _ := s.GetAccounts()
	got := map[string]float64{}
	for _, a := range as {
		got[a.ID] = a.Balance
	}
	if got["m"] != 2000 || got["privat_x"] != 7.5 {
		t.Fatalf("%v", got)
	}
}

func TestLegacyMigration(t *testing.T) {
	d := t.TempDir()
	core := filepath.Join(d, "core.db")
	raw, _ := sql.Open("sqlite", core)
	for _, q := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'DONOR', avatar_url TEXT)`,
		`CREATE TABLE user_accounts (user_id TEXT NOT NULL, account_identifier TEXT NOT NULL PRIMARY KEY, parent_identifier TEXT, note TEXT)`,
		`CREATE TABLE user_aliases (user_id TEXT NOT NULL, alias TEXT NOT NULL PRIMARY KEY)`,
		`CREATE TABLE alias_accounts (alias TEXT NOT NULL, account_identifier TEXT NOT NULL, PRIMARY KEY(alias, account_identifier))`,
		`INSERT INTO users VALUES ('u1','Док','DONOR','http://a')`,
		`INSERT INTO user_aliases VALUES ('u1','Петро Іваненко')`,
		`INSERT INTO user_aliases VALUES ('ghost','Сирота')`,
		`INSERT INTO user_accounts VALUES ('u1','` + ibMono + `','','Петро Іваненко')`,
		`INSERT INTO user_accounts VALUES ('u1','5375****1111','` + ibMono + `','Петро Іваненко')`,
		`INSERT INTO alias_accounts VALUES ('Петро Іваненко','5375****1111')`,
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	raw.Close()
	s, err := NewStorage(filepath.Join(d, "cache.db"), core)
	if err != nil {
		t.Fatal(err)
	}
	u := user(t, s)
	m := links(u, "петро іваненко")
	if len(u.Aliases) != 1 || len(m) != 2 || m[idKey("5375****1111")].ParentIdentifier != ibMono {
		t.Fatalf("%+v", u)
	}
	// повторный запуск идемпотентен
	if _, err := NewStorage(filepath.Join(d, "cache.db"), core); err != nil {
		t.Fatal(err)
	}
}

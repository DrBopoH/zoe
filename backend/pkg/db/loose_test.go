package db

import (
	"testing"

	"economic/pkg/mono"
)

func TestLooseKey(t *testing.T) {
	for in, want := range map[string]string{
		"414949****72":                  "41494972",
		"414949******0172":              "41494972",
		"4149495555550172":              "41494972",
		"4874****8644":                  "", // всего 4 открытых цифры в начале — слабый ключ не строим
		"UA323052990000000000000000965": "",
	} {
		if got := looseKey(in); got != want {
			t.Errorf("looseKey(%q)=%q want %q", in, got, want)
		}
	}
	if reliableKey("414949****72") != "" || reliableKey("414949******0172") == "" {
		t.Fatal("reliableKey")
	}
}

// Перевод «на свою карту»: получатель в выписке Привата = 414949****72, карта в кеше = 414949******0172.
func TestOwnCardTransferResolves(t *testing.T) {
	s, _, _ := newS(t)
	if err := s.SaveAccounts([]mono.AccountDTO{{ID: "p1", IBAN: "UA533052990000000000000006003", Type: "privat", MaskedPan: []string{"414949******0172"}}}, "Іваненко Петро"); err != nil {
		t.Fatal(err)
	}
	noErr(t, s.CreateUser(User{ID: "u1", Name: "Док"}))
	noErr(t, s.AddUserAlias("u1", "Іваненко Петро"))
	noErr(t, s.SaveTransactions("p1", []mono.StatementItemDTO{
		{ID: "a", Time: 10, Amount: -100, CounterName: "ІВАНЕНКО ПЕТРО ПЕТРОВИЧ", CounterIban: "414949****72"},
		{ID: "b", Time: 20, Amount: 100, CounterName: "Хтось Інший", CounterIban: "414949******0172"},
		{ID: "c", Time: 30, Amount: -5, CounterName: "Чужа Людина", CounterIban: "414949****00"},
	}))
	txs, _ := s.GetTransactions(10)
	got := map[string]string{}
	for _, x := range txs {
		got[x.ID] = x.ResolvedUserID
	}
	if got["a"] != "u1" || got["b"] != "u1" || got["c"] != "" {
		t.Fatalf("resolve by card: %v", got)
	}
	// получатель из выписки стал алиасом той же структуры (IBAN + PAN нашего счёта)
	idx, _ := s.loadCacheIndex()
	ls := idx.byAlias[aliasKey("ІВАНЕНКО ПЕТРО ПЕТРОВИЧ")]
	if len(ls) != 2 {
		t.Fatalf("own-card alias must expand to iban+pan: %+v", ls)
	}
	// чужая карта остаётся отдельным алиасом с картой как идентификатором
	if l := idx.byAlias[aliasKey("Чужа Людина")]; len(l) != 1 || l[0].ID != "414949****00" {
		t.Fatalf("foreign card: %+v", l)
	}
}

// Частичная карта из выписки уже запомнена в статике; потом кеш узнаёт полный номер —
// это та же карта: запись заменяется, дубля нет.
func TestPartialUpgradedByFullPan(t *testing.T) {
	s, _, _ := newS(t)
	noErr(t, s.SaveAccounts([]mono.AccountDTO{{ID: "m1", IBAN: ibMono, Type: "black", MaskedPan: []string{"537541******1111"}}}, "Іваненко Петро"))
	noErr(t, s.SaveTransactions("m1", []mono.StatementItemDTO{{ID: "a", Time: 10, Amount: -100, CounterName: "Іваненко Петро", CounterIban: "414949****72"}}))
	noErr(t, s.CreateUser(User{ID: "u1", Name: "Док"}))
	noErr(t, s.AddUserAlias("u1", "Іваненко Петро"))

	count := func() (n int, ids []string) {
		for _, l := range user(t, s).AliasLinks {
			if looseKey(l.AccountIdentifier) == "41494972" {
				n++
				ids = append(ids, l.AccountIdentifier)
			}
		}
		return
	}
	if n, ids := count(); n != 1 || ids[0] != "414949****72" {
		t.Fatalf("before: %v", ids)
	}
	// кеш узнал полный номер той же карты (другой счёт того же человека)
	noErr(t, s.SaveAccounts([]mono.AccountDTO{{ID: "p1", IBAN: ibPriv, Type: "privat", MaskedPan: []string{"414949******0172"}}}, "Іваненко Петро"))
	n, ids := count()
	if n != 1 || ids[0] != "414949******0172" {
		t.Fatalf("after: %v", ids)
	}
	for _, l := range user(t, s).AliasLinks {
		if l.AccountIdentifier == "414949******0172" && !l.Confirmed {
			t.Fatal("upgraded link must be confirmed")
		}
	}
}

func TestTransactionDetails(t *testing.T) {
	s, _, _ := newS(t)
	noErr(t, s.SaveAccounts([]mono.AccountDTO{{ID: "m1", IBAN: ibMono, Type: "black"}}, "X"))
	noErr(t, s.SaveTransactions("m1", []mono.StatementItemDTO{{ID: "a", Time: 5, Amount: -12345, OperationAmount: -1000, CommissionRate: 250, CashbackAmount: 30, CurrencyCode: 980}}))
	txs, err := s.GetTransactions(5)
	if err != nil || len(txs) != 1 {
		t.Fatal(txs, err)
	}
	x := txs[0]
	if x.Amount != -123.45 || x.OperationAmount != -10 || x.Commission != 2.5 || x.Cashback != 0.3 || x.IsHold {
		t.Fatalf("%+v", x)
	}
}

// Чип «чей счёт»: у операции без номера отдаём счета/карты её сырого алиаса — подтверждённые кешем первыми.
func TestAliasAcctsForTx(t *testing.T) {
	s, _, _ := newS(t)
	if err := s.SaveAccounts([]mono.AccountDTO{{ID: "m1", Type: "black", IBAN: "UA213223130000000000000006001", MaskedPan: []string{"537541******1234"}}}, "Док"); err != nil {
		t.Fatal(err)
	}
	noErr(t, s.SaveTransactions("m1", []mono.StatementItemDTO{
		{ID: "a", Time: 10, Amount: -100, CounterName: "Іван Петренко"},
		{ID: "b", Time: 20, Amount: -50, CounterName: "Іван Петренко", CounterIban: "444111******9999"},
	}))
	k := aliasKey("Іван Петренко")
	if _, err := s.coreDB.Exec(`INSERT INTO alias_accounts(alias_key, alias, account_identifier, parent_identifier) VALUES(?,?,?,?),(?,?,?,?)`,
		k, "Іван Петренко", "516875******1111", "", k, "Іван Петренко", "516875******2222", ""); err != nil {
		t.Fatal(err)
	}
	txs, _ := s.GetTransactions(10)
	for _, x := range txs {
		switch x.ID {
		case "a":
			if len(x.AliasAccts) != 3 || !x.AliasAccts[0].Confirmed || x.AliasAccts[0].ID != "444111******9999" || x.AliasAccts[1].Confirmed {
				t.Fatalf("alias accts: %+v", x.AliasAccts)
			}
		case "b":
			if len(x.AliasAccts) != 0 {
				t.Fatalf("tx with its own counter_iban must not carry alias accts: %+v", x.AliasAccts)
			}
		}
	}
}

// Перевод между своими счетами: Raif-сторона называет карту NovaPay текстом («5175 47** **** 0170»), NovaPay-сторона — маской в counter_iban.
func TestOwnAccountDetected(t *testing.T) {
	s, _, _ := newS(t)
	if err := s.SaveAccounts([]mono.AccountDTO{{ID: "raif_1", IBAN: "UA583003350000000026001234567", Type: "raif", MaskedPan: []string{"4149****5262"}}}, "Raif"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveImportedAccount(mono.AccountDTO{ID: "novapay_1", IBAN: "UA129358710000067401001111111", Type: "novapay", MaskedPan: []string{"5175****0170"}}, "NovaPay"); err != nil {
		t.Fatal(err)
	}
	noErr(t, s.SaveTransactions("raif_1", []mono.StatementItemDTO{
		{ID: "a", Time: 10, Amount: -100, CounterName: "5175 47** **** 0170", Description: "Грошові перекази: 5175 47** **** 0170"},
		{ID: "b", Time: 20, Amount: -5, CounterName: "Кафе", Description: "Кава"},
		{ID: "c", Time: 30, Amount: -7, CounterName: "Хтось", Description: "Переказ 4149 12** **** 5262"}, // это своя карта самой операции — не «другой счёт»
	}))
	noErr(t, s.SaveTransactions("novapay_1", []mono.StatementItemDTO{
		{ID: "d", Time: 11, Amount: 100, CounterName: "Ivanenko", CounterIban: "4149****5262",
			Description: "Зарахування на карту № 517547******0170 Іваненко, 6536, від Ivanenko|KYIV 414951***5262"},
		{ID: "e", Time: 12, Amount: 3, CounterName: "Чужий", Description: "Зарахування на карту № 517547******0170 Іваненко"},
	}))
	txs, _ := s.GetTransactions(10)
	got := map[string]string{}
	for _, x := range txs {
		got[x.ID] = x.OwnAccountID
	}
	if got["a"] != "novapay_1" || got["d"] != "raif_1" || got["b"] != "" || got["c"] != "" || got["e"] != "" {
		t.Fatalf("own: %+v", got)
	}
}

// Перевод назван номером своей карты: Лицо подтягивается по владельцу этого счёта.
func TestOwnAccountResolvesOwner(t *testing.T) {
	s, _, _ := newS(t)
	noErr(t, s.SaveAccounts([]mono.AccountDTO{{ID: "raif_1", IBAN: "UA583003350000000026001234567", Type: "raif", MaskedPan: []string{"4149****5262"}}}, "Raif"))
	noErr(t, s.SaveImportedAccount(mono.AccountDTO{ID: "novapay_1", IBAN: "UA129358710000067401001111111", Type: "novapay", MaskedPan: []string{"5175****0170"}}, "NovaPay"))
	noErr(t, s.CreateUser(User{ID: "u1", Name: "Док"}))
	noErr(t, s.AddUserAlias("u1", "Док"))
	noErr(t, s.LinkAccount("u1", "Док", "5175****0170", ""))
	noErr(t, s.SaveTransactions("raif_1", []mono.StatementItemDTO{{ID: "a", Time: 10, Amount: -100, CounterName: "5175 47** **** 0170"}}))
	txs, _ := s.GetTransactions(10)
	if len(txs) != 1 || txs[0].OwnAccountID != "novapay_1" || txs[0].ResolvedUserID != "u1" {
		t.Fatalf("%+v", txs)
	}
}

// Номера в операции нет, но у алиаса ровно один наш счёт (IBAN и карта одного счёта) — это перевод на него.
func TestOwnAccountByAlias(t *testing.T) {
	s, _, _ := newS(t)
	noErr(t, s.SaveAccounts([]mono.AccountDTO{{ID: "raif_1", IBAN: "UA583003350000000026001234567", Type: "raif", MaskedPan: []string{"4149****5262"}}}, "Raif"))
	noErr(t, s.SaveAccounts([]mono.AccountDTO{{ID: "mono_1", IBAN: "UA443220010000026001111119976", Type: "black", MaskedPan: []string{"4874****8644"}}}, "Mono"))
	noErr(t, s.CreateUser(User{ID: "u1", Name: "Док"}))
	noErr(t, s.AddUserAlias("u1", "Док"))
	noErr(t, s.LinkAccount("u1", "Док", "UA443220010000026001111119976", ""))
	noErr(t, s.LinkAccount("u1", "Док", "4874****8644", "UA443220010000026001111119976"))
	noErr(t, s.SaveTransactions("raif_1", []mono.StatementItemDTO{{ID: "a", Time: 10, Amount: 580, CounterName: "Док"}}))
	txs, _ := s.GetTransactions(10)
	if len(txs) != 1 || txs[0].OwnAccountID != "mono_1" {
		t.Fatalf("%+v", txs)
	}
}

// noErr — подготовка данных в тесте: упавшая запись должна ронять тест сразу, а не проявляться странным результатом дальше.
func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

package db

import "testing"

func hasID(l []AliasLink, id string) (AliasLink, bool) {
	for _, x := range l {
		if idKey(x.AccountIdentifier) == idKey(id) {
			return x, true
		}
	}
	return AliasLink{}, false
}

func structOf(t *testing.T, s *Storage, alias string) []AliasLink {
	st, err := s.GetAliasStructures()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range st {
		if aliasKey(a.Alias) == aliasKey(alias) {
			return a.Accounts
		}
	}
	return nil
}

// PAN→IBAN выучен на одном алиасе — другой алиас с одним лишь IBAN получает карты сам.
func TestGlobalIbanPans(t *testing.T) {
	s, _, _ := newS(t)
	ib := "UA903052990000026205999999999"
	if err := s.LinkAccount("", "Alice", "4149490000001111", ib); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkAccount("", "Alice", ib, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkAccount("", "Bob", ib, ""); err != nil {
		t.Fatal(err)
	}
	l, ok := hasID(structOf(t, s, "Bob"), "4149490000001111")
	if !ok || idKey(l.ParentIdentifier) != idKey(ib) {
		t.Fatalf("Bob must get card via IBAN: %+v", structOf(t, s, "Bob"))
	}
	// ручное снятие карты у Bob не трогает Alice и не воскресает
	if err := s.UnlinkAccount("Bob", "4149490000001111"); err != nil {
		t.Fatal(err)
	}
	if _, ok := hasID(structOf(t, s, "Bob"), "4149490000001111"); ok {
		t.Fatal("unlinked card resurrected")
	}
	if _, ok := hasID(structOf(t, s, "Alice"), "4149490000001111"); !ok {
		t.Fatal("Alice lost card")
	}
	// возврат вручную снимает бан
	if err := s.LinkAccount("", "Bob", "4149490000001111", ib); err != nil {
		t.Fatal(err)
	}
	if _, ok := hasID(structOf(t, s, "Bob"), "4149490000001111"); !ok {
		t.Fatal("manual relink failed")
	}
}

// карты из кеша сохраняются глобально и переживают очистку кеша.
func TestIbanPansSurviveCacheWipe(t *testing.T) {
	s, _, _ := newS(t)
	fillCache(t, s)
	if err := s.LinkAccount("", "Zed", ibMono, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.cacheDB.Exec(`DELETE FROM accounts`); err != nil {
		t.Fatal(err)
	}
	if _, ok := hasID(structOf(t, s, "Zed"), "537541******1111"); !ok {
		t.Fatalf("cards lost after wipe: %+v", structOf(t, s, "Zed"))
	}
}

// убрали IBAN целиком (карты забанены каскадом) → добавили IBAN обратно: карты возвращаются сами.
func TestRelinkIbanRestoresPans(t *testing.T) {
	s, _, _ := newS(t)
	ib := "UA903052990000026205888888888"
	pan := "4149490000002222"
	if err := s.LinkAccount("", "Carol", pan, ib); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkAccount("", "Carol", ib, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.UnlinkAccount("Carol", ib); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkAccount("", "Carol", ib, ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := hasID(structOf(t, s, "Carol"), pan); !ok {
		t.Fatalf("card must come back with IBAN: %+v", structOf(t, s, "Carol"))
	}
}

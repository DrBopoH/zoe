package source

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func slotOf(y int, m time.Month, d, h int) int64 {
	return time.Date(y, m, d, h, 0, 0, 0, time.UTC).Unix()
}

func TestAppendTicksKeepsMonthFileAndReplacesSameSlot(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	h := int64(3600)
	a := slotOf(2026, 11, 16, 12)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.AppendTicks(SetChainTicks, h, a, map[string]float64{"p2p_ask": 41.2, "fee_tron_usd": 0.5}))
	must(s.AppendTicks(SetChainTicks, h, a, map[string]float64{"p2p_ask": 41.3})) // тот же час: значение заменяется
	must(s.AppendTicks(SetChainTicks, h, a+h, map[string]float64{"p2p_ask": 41.5, "fee_tron_usd": 0.6}))

	files, bad := s.ListEconomy(SetChainTicks)
	if len(bad) != 0 || len(files) != 1 {
		t.Fatalf("месяц — один файл, прежние версии удаляются: %d файлов %v", len(files), bad)
	}
	f := files[0].File
	if f.Group != GroupEconomy || f.Kind != KindDataset || f.Data.Name != "chain-ticks-2026-11" || f.Count() != 4 || f.From != a || f.To != a+h {
		t.Fatalf("файл: %+v", f)
	}
	got := s.ReadTicks(SetChainTicks, "p2p_ask", 0)
	if len(got) != 2 || got[0] != (Tick{a, 41.3}) || got[1] != (Tick{a + h, 41.5}) {
		t.Fatalf("p2p_ask: %+v", got)
	}
	if got := s.ReadTicks(SetChainTicks, "p2p_ask", a+h); len(got) != 1 {
		t.Fatalf("since: %+v", got)
	}
	if got := s.ReadTicks(SetChainTicks, "нет_такого", 0); len(got) != 0 {
		t.Fatalf("нет ряда: %+v", got)
	}
	if s.LastTick(SetChainTicks) != a+h {
		t.Fatalf("последний слот: %d", s.LastTick(SetChainTicks))
	}
}

func TestAppendTicksNewMonthNewFileAndSlotInsertedInOrder(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	h := int64(3600)
	nov := slotOf(2026, 11, 30, 23)
	dec := slotOf(2026, 12, 1, 0)
	for _, at := range []int64{dec, nov, nov - 2*h, nov - h} { // порядок записи не по времени
		if err := s.AppendTicks(SetChainTicks, h, at, map[string]float64{"dom_btc": float64(at % 100)}); err != nil {
			t.Fatal(err)
		}
	}
	files, _ := s.ListEconomy(SetChainTicks)
	if len(files) != 2 || files[0].File.Data.Name != "chain-ticks-2026-11" || files[1].File.Data.Name != "chain-ticks-2026-12" {
		t.Fatalf("два месяца — два файла: %d", len(files))
	}
	got := s.ReadTicks(SetChainTicks, "dom_btc", 0)
	if len(got) != 4 {
		t.Fatalf("точки всех месяцев: %+v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].T >= got[i].T {
			t.Fatalf("по возрастанию времени: %+v", got)
		}
	}
	if s.LastTick(SetChainTicks) != dec {
		t.Fatalf("последний слот: %d", s.LastTick(SetChainTicks))
	}
}

func TestEconomyIsInvisibleToAccountsAndLegacyFilesStay(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	acc := &File{Name: "stmt.csv", Origin: OriginCSV, Kind: KindBank, AccountID: "acc1", Card: "1234"}
	accPath, added, err := s.Put(acc)
	if err != nil || !added {
		t.Fatalf("файл Счетов: %v %v", added, err)
	}
	if filepath.Dir(filepath.Dir(accPath)) != s.Dir { // как раньше: <каталог>/<карта>/файл, без группы
		t.Fatalf("Счета лежат там же, где лежали: %s", accPath)
	}
	if err := s.AppendTicks(SetChainTicks, 3600, slotOf(2026, 11, 16, 12), map[string]float64{"p2p_ask": 41}); err != nil {
		t.Fatal(err)
	}
	list, bad := s.List()
	if len(bad) != 0 || len(list) != 1 || list[0].File.AccountID != "acc1" {
		t.Fatalf("List() — только Счета: %+v %v", list, bad)
	}
	eco, _ := s.ListEconomy("")
	if len(eco) != 1 || eco[0].File.Card != SetChainTicks {
		t.Fatalf("ListEconomy: %+v", eco)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, GroupEconomy, SetChainTicks)); err != nil {
		t.Fatalf("папка набора: %v", err)
	}
}

func TestEconomyFileValidationAndDedup(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if _, _, err := s.Put(&File{Group: GroupEconomy, Kind: KindDataset, Card: "x"}); err == nil {
		t.Fatal("набор без данных — ошибка")
	}
	if _, _, err := s.Put(&File{Group: "чужая", AccountID: "a", Kind: KindBank}); err == nil {
		t.Fatal("неизвестная группа — ошибка")
	}
	a := slotOf(2026, 11, 16, 12)
	vals := map[string]float64{"p2p_ask": 41}
	if err := s.AppendTicks(SetChainTicks, 3600, a, vals); err != nil {
		t.Fatal(err)
	}
	before, _ := s.ListEconomy("")
	if err := s.AppendTicks(SetChainTicks, 3600, a, vals); err != nil { // те же данные: дубль по отпечатку, файл не меняется
		t.Fatal(err)
	}
	after, _ := s.ListEconomy("")
	if len(after) != 1 || after[0].Path != before[0].Path || after[0].File.Hash != before[0].File.Hash {
		t.Fatalf("дубль не плодит файлы: %+v", after)
	}
	if err := s.AppendTicks(SetChainTicks, 3600, a, nil); err != nil {
		t.Fatalf("пустой набор — ничего не пишется: %v", err)
	}
}

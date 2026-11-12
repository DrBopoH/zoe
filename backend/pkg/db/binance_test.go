package db

import (
	"testing"
)

const (
	bnFile    = "Binance Тест.csv"
	bnProfile = "BN1000000001"
)

func bnRows() []BinanceRow {
	return []BinanceRow{
		{T: 1_700_000_000_000, Account: "Spot", Op: "Deposit", Asset: "USDT", Delta: 100, Kind: "deposit"},
		{T: 1_700_000_100_000, Account: "Spot", Op: "Transaction Buy", Asset: "BTC", Delta: 0.001},
		{T: 1_700_000_100_000, Account: "Spot", Op: "Transaction Spend", Asset: "USDT", Delta: -40},
		{T: 1_700_000_100_000, Account: "Spot", Op: "Transaction Spend", Asset: "USDT", Delta: -40}, // две одинаковые строки в одну секунду — обе настоящие
	}
}

// Binance — такой же кеш, как банки: счёт и журнал из выписки, вайп кеша всё сносит, в core.db ничего.
func TestBinanceLivesInCacheOnly(t *testing.T) {
	s, _, _ := newS(t)
	if accs, _ := s.GetAllAccounts(); len(accs) != 0 {
		t.Fatalf("до выписки счёта Binance нет: %+v", accs)
	}
	if err := s.SaveBinanceStatement(bnFile, bnProfile, bnRows()); err != nil {
		t.Fatal(err)
	}
	accs, _ := s.GetAllAccounts()
	if len(accs) != 1 {
		t.Fatalf("счета: %+v", accs)
	}
	a := accs[0]
	if a.ID != BinanceAccountID || a.Type != BinanceAccountType || a.Name != bnFile+" (binance)" || a.BankKey != "binance" || !a.HasDetail || a.Currency != 980 || a.OpenedAt != 1_700_000_000 {
		t.Fatalf("счёт: %+v", a)
	}
	rows, _ := s.BinanceLedger()
	if len(rows) != 4 {
		t.Fatalf("журнал: %d строк", len(rows))
	}
	for _, n := range legacyBinanceCore {
		if tableExists(s.coreDB, n) {
			t.Fatalf("в core.db таблица %s", n)
		}
	}
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if s.HasBinance() {
		t.Fatal("после вайпа журнал остался")
	}
	if accs, _ := s.GetAllAccounts(); len(accs) != 0 {
		t.Fatalf("после вайпа остался счёт: %+v", accs)
	}
}

// Выписки за разные периоды перекрываются: повторная и пересекающаяся загрузка не задваивает строки,
// а две одинаковые строки внутри одной выписки сохраняются обе.
func TestBinanceStatementsOverlapWithoutDuplicates(t *testing.T) {
	s, _, _ := newS(t)
	if err := s.SaveBinanceStatement(bnFile, bnProfile, bnRows()); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBinanceStatement(bnFile, bnProfile, bnRows()); err != nil {
		t.Fatal(err)
	}
	more := append(bnRows()[2:], BinanceRow{T: 1_700_000_200_000, Account: "Spot", Op: "Withdraw", Asset: "USDT", Delta: -5, Kind: "withdraw"})
	if err := s.SaveBinanceStatement("Binance Тест 2.csv", bnProfile, more); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.BinanceLedger()
	if len(rows) != 5 {
		t.Fatalf("ждали 5 строк (4 + новая), получили %d", len(rows))
	}
	if accs, _ := s.GetAllAccounts(); len(accs) != 1 || accs[0].Name != "Binance Тест 2.csv (binance)" {
		t.Fatalf("счёт один, имя — по последнему файлу: %+v", accs)
	}
}

// Идентификатор профиля известен кешу из выписки: он сразу в списке счетов (алиас файла получит его сам, без ручной привязки),
// а исчезает вместе с кешем.
func TestBinanceIdentityComesFromStatement(t *testing.T) {
	s, _, _ := newS(t)
	if err := s.SaveBinanceStatement(bnFile, bnProfile, bnRows()); err != nil {
		t.Fatal(err)
	}
	accs, _ := s.GetAllAccounts()
	if len(accs) != 1 || accs[0].IBAN != bnProfile {
		t.Fatalf("идентификатор: %+v", accs)
	}
	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if s.BinanceProfile() != "" {
		t.Fatal("после вайпа профиль остался")
	}
}

// Оценка портфеля (минутный ряд и баланс) пишется отдельно от журнала; новая выписка сбрасывает ряд до пересборки.
func TestBinanceSeriesIsRebuiltAfterNewStatement(t *testing.T) {
	s, _, _ := newS(t)
	_ = s.SaveBinanceStatement(bnFile, bnProfile, bnRows())
	t0 := int64(1_700_000_000) / 60 * 60
	if err := s.SaveBinance(BinanceSave{Portfolio: 1150, Start: t0, Vals: []float64{100, 110, 90}, Replace: true, Detail: map[string]any{"total": 1150}}); err != nil {
		t.Fatal(err)
	}
	if accs, _ := s.GetAllAccounts(); accs[0].Balance != 1150 {
		t.Fatalf("баланс: %+v", accs[0])
	}
	if len(s.BinanceDetail()) == 0 {
		t.Fatal("разбор не сохранён")
	}
	_ = s.SaveBinanceStatement(bnFile, bnProfile, bnRows())
	if _, _, n := s.BinanceSeriesInfo(); n != 0 {
		t.Fatal("ряд должен быть сброшен до пересборки")
	}
}

// Внешние движения журнала становятся операциями счёта один раз.
func TestBinanceExternalFlowsBecomeTransactions(t *testing.T) {
	s, _, _ := newS(t)
	_ = s.SaveBinanceStatement(bnFile, bnProfile, bnRows())
	fl, err := s.BinanceUnvaluedFlows([]string{"deposit", "withdraw"})
	if err != nil || len(fl) != 1 || fl[0].Asset != "USDT" || fl[0].Delta != 100 {
		t.Fatalf("внешние движения: %+v %v", fl, err)
	}
	if err := s.SaveBinanceTxs([]BinanceTx{{FlowID: fl[0].ID, T: fl[0].T, Desc: "Binance: депозит", UAH: 4000}}); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.BinanceUnvaluedFlows([]string{"deposit"}); len(again) != 0 {
		t.Fatalf("оценённое не должно возвращаться: %+v", again)
	}
	txs, _ := s.GetAllTransactions(10)
	if len(txs) != 1 || !txs[0].Internal || txs[0].Amount != 4000 {
		t.Fatalf("операции: %+v", txs)
	}
}

func TestBinanceMinutesCandles(t *testing.T) {
	s, _, _ := newS(t)
	t0 := int64(1_700_000_000) / 60 * 60
	vals := []float64{100, 110, 90, 95, 95, 120, 118, 118, 118, 130}
	// Origin = первая точка: как ряд без истории движений
	if err := s.SaveBinance(BinanceSave{Portfolio: 130, Start: t0, Vals: vals, Replace: true, Origin: 100}); err != nil {
		t.Fatal(err)
	}
	// корзины: до ряда | [t0,t0+5m) | [t0+5m,t0+10m) | после ряда (плоская)
	starts := []int64{t0 - 600, t0, t0 + 300, t0 + 600}
	got, has, err := s.BinanceCandles(starts, t0+900)
	if err != nil {
		t.Fatal(err)
	}
	if has[0] {
		t.Fatalf("до ряда свечи нет: %v", got[0])
	}
	if got[1] != [4]float64{100, 110, 90, 95} {
		t.Fatalf("корзина 1: %v", got[1])
	}
	if got[2] != [4]float64{95, 130, 95, 130} {
		t.Fatalf("корзина 2: %v", got[2])
	}
	if !has[3] || got[3] != [4]float64{130, 130, 130, 130} {
		t.Fatalf("после ряда — плоская: %v", got[3])
	}
	if err := s.SaveBinance(BinanceSave{Portfolio: 140, Start: t0 + 600, Vals: []float64{140}}); err != nil {
		t.Fatal(err)
	}
	if f, l, n := s.BinanceSeriesInfo(); f != t0 || l != t0+600 || n != 11 {
		t.Fatalf("info: %d %d %d", f, l, n)
	}
}

// Алиас файла выписки привязан к лицу → профиль Binance попадает в статику (единственным путём «кеш → статика»)
// и переживает сброс кеша: карта в лице не исчезает. Порядок «выписка → лицо» и «лицо → выписка» одинаков.
func TestBinanceProfileSurvivesWipeForAttachedAlias(t *testing.T) {
	const alias, profile = "Крипто Іваненко Петро.csv", "BN1245940041"
	rows := []BinanceRow{{T: 1000, Account: "Spot", Op: "Deposit", Asset: "USDT", Delta: 10, Kind: "deposit"}}
	links := func(s *Storage) []AliasLink {
		us, err := s.GetUsers()
		if err != nil || len(us) != 1 {
			t.Fatalf("лица: %v %v", us, err)
		}
		return us[0].AliasLinks
	}
	for _, statementFirst := range []bool{true, false} {
		s, _, _ := newS(t)
		if err := s.CreateUser(User{ID: "u1", Name: "Петро"}); err != nil {
			t.Fatal(err)
		}
		if statementFirst {
			if err := s.SaveBinanceStatement(alias, profile, rows); err != nil {
				t.Fatal(err)
			}
			if err := s.AddUserAlias("u1", alias); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := s.AddUserAlias("u1", alias); err != nil {
				t.Fatal(err)
			}
			if err := s.SaveBinanceStatement(alias, profile, rows); err != nil {
				t.Fatal(err)
			}
		}
		if l := links(s); len(l) != 1 || l[0].AccountIdentifier != profile {
			t.Fatalf("statementFirst=%v: профиль не попал в статику: %+v", statementFirst, l)
		}
		if err := s.WipeCache(); err != nil {
			t.Fatal(err)
		}
		if l := links(s); len(l) != 1 || l[0].AccountIdentifier != profile {
			t.Fatalf("statementFirst=%v: после сброса кеша связи нет: %+v", statementFirst, l)
		}
	}
}

// Кеш, посчитанный по старым правилам учёта, при открытии теряет ряд и операции (журнал остаётся); по текущим — нет.
func TestBinanceModelMigrationDropsStaleSeries(t *testing.T) {
	s, cachePath, corePath := newS(t)
	rows := []BinanceRow{{T: 1000, Account: "Spot", Op: "Deposit", Asset: "USDT", Delta: 10, Kind: "deposit"}}
	if err := s.SaveBinanceStatement("b.csv", "BN1000001", rows); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBinance(BinanceSave{Portfolio: 1, Start: 60, Vals: []float64{1, 2, 3}, Replace: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBinanceTxs([]BinanceTx{{FlowID: "x", T: 1000, Desc: "d", UAH: 1}}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err := NewStorage(cachePath, corePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, n := s.BinanceSeriesInfo(); n != 3 {
		t.Fatalf("ряд текущей модели жив: %d", n)
	}
	if _, err := s.cacheDB.Exec(`UPDATE binance_state SET v='1' WHERE k='model'`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewStorage(cachePath, corePath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var tx int
	_ = s.cacheDB.QueryRow(`SELECT COUNT(*) FROM transactions WHERE id LIKE 'binance_%'`).Scan(&tx)
	if _, _, n := s.BinanceSeriesInfo(); n != 0 || tx != 0 || !s.HasBinance() {
		t.Fatalf("старая модель: ряд %d, операций %d, журнал %v", n, tx, s.HasBinance())
	}
}

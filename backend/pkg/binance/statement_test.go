package binance

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) *Statement {
	t.Helper()
	raw, err := os.ReadFile("testdata/statement.csv")
	if err != nil {
		t.Fatal(err)
	}
	if !IsStatement(raw) {
		t.Fatal("фикстура должна распознаваться как выписка")
	}
	st, err := ParseStatement(strings.NewReader(string(raw)), StatementZone("Binance-Історія-транзакцій-202611111427(UTC+2)-part1-of1.csv"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestParseKeepsEveryRow(t *testing.T) {
	st := fixture(t)
	if st.UserID != "1000000001" || ProfileID(st.UserID) != "BN1000000001" {
		t.Fatalf("профиль: %q", st.UserID)
	}
	if len(st.Rows) != 66 {
		t.Fatalf("все строки должны быть прочитаны, без белого списка: %d", len(st.Rows))
	}
	first := st.Rows[0]
	// 08:05:59 по UTC+2 — это 06:05:59 UTC
	if first.Account != "Funding" || first.Op != "P2P Trading" || first.Asset != "USDT" || first.Delta != 25.06 || first.T != 1779170759000 {
		t.Fatalf("первая строка: %+v", first)
	}
}

func TestStatementZoneFromFileName(t *testing.T) {
	if off := offsetOf(StatementZone("x(UTC+2)y")); off != 2*3600 {
		t.Fatalf("UTC+2: %d", off)
	}
	if off := offsetOf(StatementZone("x(UTC-3:30)y")); off != -(3*3600 + 1800) {
		t.Fatalf("UTC-3:30: %d", off)
	}
	if off := offsetOf(StatementZone("Мій файл.csv")); off != 0 {
		t.Fatalf("без пояса — UTC: %d", off)
	}
}

func TestOneUserPerStatement(t *testing.T) {
	_, err := ParseStatement(strings.NewReader("Ідентифікатор користувача,Час,Акаунт,Операція,Монета,Змінити,Примітка\n"+
		"1,2026-01-01 00:00:00,Spot,Deposit,USDT,5,\n2,2026-01-01 00:00:00,Spot,Deposit,USDT,5,\n"), nil)
	if err == nil {
		t.Fatal("два пользователя в одном файле — ошибка")
	}
	if _, err := ParseStatement(strings.NewReader("a,b\n1,2\n"), nil); err == nil {
		t.Fatal("чужой CSV — ошибка")
	}
}

func TestKindsOfOperations(t *testing.T) {
	cases := []struct {
		r    Row
		want string
	}{
		{Row{Account: "Spot", Op: "Deposit"}, KindDeposit},
		{Row{Account: "Funding", Op: "P2P Trading"}, KindP2P},
		{Row{Account: "Spot", Op: "Cashback Voucher"}, KindBonus},
		{Row{Account: "Spot", Op: "Simple Earn Flexible Interest"}, KindDividend},
		{Row{Account: "Spot", Op: "Simple Earn Locked Rewards"}, KindDividend},
		{Row{Account: "Spot", Op: "Transfer Between Spot and Strategy"}, ""}, // переводы между кошельками — внутри портфеля
		{Row{Account: "Strategy", Op: "Transfer Between Spot and Strategy"}, ""},
		{Row{Account: "Spot", Op: "Transfer Between Spot and UM Futures"}, ""},
		{Row{Account: "Spot", Op: "Transaction Buy"}, ""},
		{Row{Account: "Spot", Op: "Binance Convert"}, ""},
		{Row{Account: "Spot", Op: "Simple Earn Flexible Subscription"}, ""},
	}
	for _, c := range cases {
		if got := Kind(c.r); got != c.want {
			t.Errorf("%s/%s: %q, ждали %q", c.r.Account, c.r.Op, got, c.want)
		}
	}
}

// Портфель = Spot + Funding + Strategy + Futures + Earn. Переводы между ними внутри журнала не меняют сумму «по монетам»:
// остатки из HoldingsOf и нарастающий итог Flows обязаны совпадать — иначе график разошёлся бы с модалкой.
func TestHoldingsAgreeWithFlows(t *testing.T) {
	st := fixture(t)
	total := map[string]float64{}
	for _, f := range Flows(st.Rows) {
		total[f.Asset] += f.Delta
	}
	for a, w := range HoldingsOf(st.Rows) {
		if math.Abs(w.Total()-total[a]) > 1e-9 {
			t.Errorf("%s: кошельки %.9f, движения %.9f", a, w.Total(), total[a])
		}
		delete(total, a)
	}
	for a, v := range total {
		if math.Abs(v) > 1e-9 {
			t.Errorf("%s: движения %.9f, а в остатках нет", a, v)
		}
	}
}

// Деньги в боте и на фьючерсах остаются в портфеле: перевод туда не «уход», а результат их работы двигает остаток.
func TestStrategyAndFuturesAreWalletsOfPortfolio(t *testing.T) {
	rows := []Row{
		{T: 1, Account: "Spot", Op: "Transfer Between Spot and Strategy", Asset: "USDT", Delta: -10},
		{T: 1, Account: "Strategy", Op: "Transfer Between Spot and Strategy", Asset: "USDT", Delta: 10},
		{T: 2, Account: "Strategy", Op: "Transaction Buy", Asset: "BNB", Delta: 1},
		{T: 2, Account: "Strategy", Op: "Transaction Spend", Asset: "USDT", Delta: -6},
		{T: 3, Account: "USD-M Futures", Op: "Realized Profit and Loss", Asset: "USDT", Delta: 4},
		{T: 4, Account: "Spot", Op: "Simple Earn Flexible Subscription", Asset: "USDT", Delta: -5},
	}
	fl := Flows(rows) // перевода нет, остальное двигает портфель
	if len(fl) != 3 || fl[0].Asset != "BNB" || fl[1].Delta != -6 || fl[2].Delta != 4 {
		t.Fatalf("движения: %+v", fl)
	}
	h := HoldingsOf(rows)
	if w := h["USDT"]; w == nil || w.Spot != -15 || w.Strategy != 4 || w.Futures != 4 || w.Earn != 5 || w.Total() != -2 {
		t.Fatalf("кошельки USDT: %+v", h["USDT"])
	}
	if w := h["BNB"]; w == nil || w.Strategy != 1 {
		t.Fatalf("BNB бота в портфеле: %+v", h["BNB"])
	}
}

func offsetOf(l *time.Location) int { _, o := time.Unix(0, 0).In(l).Zone(); return o }

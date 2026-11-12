package binance

import "testing"

func TestMovesPairsAndEarn(t *testing.T) {
	rows := []Row{
		{T: 1000, Account: "Spot", Op: "Transfer Between Spot and Strategy", Asset: "USDT", Delta: -50},
		{T: 1000, Account: "Strategy", Op: "Transfer Between Spot and Strategy", Asset: "USDT", Delta: 50},
		{T: 2000, Account: "USD-M Futures", Op: "Transfer Between Spot and UM Futures", Asset: "USDT", Delta: 20},
		{T: 2000, Account: "Spot", Op: "Transfer Between Spot and UM Futures", Asset: "USDT", Delta: -20},
		{T: 3000, Account: "Spot", Op: "Simple Earn Flexible Subscription", Asset: "USDT", Delta: -10},
		{T: 4000, Account: "Spot", Op: "Simple Earn Flexible Redemption", Asset: "USDT", Delta: 10},
		{T: 5000, Account: "Spot", Op: "Transfer Between Spot and Funding", Asset: "USDT", Delta: 25}, // вторая половина не попала в выписку
		{T: 6000, Account: "Spot", Op: "Simple Earn Flexible Interest", Asset: "USDT", Delta: 0.01},   // начисление — не перемещение
	}
	got := Moves(rows)
	type m struct{ from, to string }
	want := map[int64]m{1000: {"Spot", "Strategy"}, 2000: {"Spot", "USD-M Futures"}, 3000: {"Spot", "Earn"}, 4000: {"Earn", "Spot"}, 5000: {"Funding", "Spot"}}
	if len(got) != len(want) {
		t.Fatalf("ждали %d перемещений, есть %d: %+v", len(want), len(got), got)
	}
	for _, x := range got {
		if w := want[x.T]; x.From != w.from || x.To != w.to || x.Qty <= 0 {
			t.Fatalf("t=%d: %+v, ждали %v", x.T, x, w)
		}
	}
	if got[0].T < got[len(got)-1].T {
		t.Fatal("новые сверху")
	}
}

// Половины перевода с разницей в секунду — один перевод, а не два.
func TestMovesPairWithSecondApart(t *testing.T) {
	rows := []Row{
		{T: 1000, Account: "Spot", Op: "Transfer Between Spot and Funding", Asset: "USDT", Delta: 25.06},
		{T: 2000, Account: "Funding", Op: "Transfer Between Spot and Funding", Asset: "USDT", Delta: -25.06},
	}
	got := Moves(rows)
	if len(got) != 1 || got[0].From != "Funding" || got[0].To != "Spot" || got[0].Qty != 25.06 {
		t.Fatalf("%+v", got)
	}
}

// Автоподписка: проценты пришли и через время тем же числом ушли в Earn — это не отдельный перевод. Ручная подписка на другую сумму — перевод.
func TestMovesMarkAutoSubscription(t *testing.T) {
	h := int64(3600_000)
	rows := []Row{
		{T: 1 * h, Account: "Spot", Op: "Simple Earn Flexible Interest", Asset: "USDT", Delta: 0.0111},
		{T: 2 * h, Account: "Spot", Op: "Simple Earn Flexible Subscription", Asset: "USDT", Delta: -0.0111},  // автоподписка
		{T: 3 * h, Account: "Spot", Op: "Simple Earn Flexible Subscription", Asset: "USDT", Delta: -50},      // ручная
		{T: 90 * h, Account: "Spot", Op: "Simple Earn Flexible Subscription", Asset: "USDT", Delta: -0.0111}, // слишком поздно для начисления
		{T: 100 * h, Account: "Spot", Op: "Simple Earn Locked Rewards", Asset: "SOL", Delta: 0.2},
		{T: 101 * h, Account: "Spot", Op: "Simple Earn Locked Subscription", Asset: "SOL", Delta: -0.2}, // автоподписка наград
	}
	auto := map[int64]bool{}
	for _, m := range Moves(rows) {
		auto[m.T/h] = m.Auto
	}
	if !auto[2] || auto[3] || auto[90] || !auto[101] {
		t.Fatalf("auto по часам: %v", auto)
	}
}

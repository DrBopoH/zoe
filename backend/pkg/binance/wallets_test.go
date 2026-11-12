package binance

import "testing"

func TestWalletLines(t *testing.T) {
	rows := []Row{
		{T: 1000, Account: "Spot", Op: "Deposit", Asset: "USDT", Delta: 100},
		// две покупки SOL: старая дешёвая и свежая дорогая; держим 1 SOL — покрывает свежая
		{T: 2000, Account: "Spot", Op: "Transaction Buy", Asset: "SOL", Delta: 1},
		{T: 2000, Account: "Spot", Op: "Transaction Spend", Asset: "USDT", Delta: -10},
		{T: 3000, Account: "Spot", Op: "Transaction Buy", Asset: "SOL", Delta: 1},
		{T: 3000, Account: "Spot", Op: "Transaction Spend", Asset: "USDT", Delta: -20},
		{T: 3500, Account: "Spot", Op: "Transaction Sold", Asset: "SOL", Delta: -1},
		{T: 3500, Account: "Spot", Op: "Transaction Revenue", Asset: "USDT", Delta: 15},
		{T: 4000, Account: "Spot", Op: "Simple Earn Flexible Subscription", Asset: "USDT", Delta: -50},
		{T: 5000, Account: "Spot", Op: "Simple Earn Flexible Interest", Asset: "USDT", Delta: 0.5},
	}
	prices := map[string]float64{"USDTUAH": 40, "SOLUSDT": 25}
	h := HoldingsOf(rows)
	lines := WalletLines(h, rows, prices)
	by := map[string]WalletLine{}
	for _, l := range lines {
		by[l.Wallet] = l
	}
	if _, ok := by[WalletFutures]; ok {
		t.Fatal("пустой кошелёк должен скрываться")
	}
	spot := by[WalletSpot]
	var sol, usdt WalletItem
	for _, it := range spot.Items {
		if it.Asset == "SOL" {
			sol = it
		}
		if it.Asset == "USDT" {
			usdt = it
		}
	}
	// держим 1 SOL: свежая покупка стоила 20 USDT, сейчас 25 → +5 USDT = +200 ₴
	if sol.Qty != 1 || sol.Delta == nil || *sol.Delta != 200 || sol.DeltaKind != "pnl" || sol.ValueUAH != 1000 {
		t.Fatalf("SOL: %+v", sol)
	}
	if usdt.Delta != nil {
		t.Fatalf("у стейблкоина результата нет: %+v", usdt)
	}
	earn := by[WalletEarn]
	if len(earn.Items) != 1 || earn.Items[0].Qty != 50 || earn.Items[0].Delta == nil || *earn.Items[0].Delta != 20 || earn.Items[0].DeltaKind != "last" {
		t.Fatalf("Earn: %+v", earn)
	}
}

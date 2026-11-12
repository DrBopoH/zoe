package binance

import "testing"

func TestTradesPairBuySell(t *testing.T) {
	rows := []Row{
		{T: 1000, Account: "Spot", Op: "Transaction Buy", Asset: "BTC", Delta: 0.001},
		{T: 1000, Account: "Spot", Op: "Transaction Spend", Asset: "USDT", Delta: -40},
		{T: 1000, Account: "Spot", Op: "Transaction Fee", Asset: "BTC", Delta: -0.000001},
		{T: 2000, Account: "Spot", Op: "Transaction Sold", Asset: "BTC", Delta: -0.001},
		{T: 2000, Account: "Spot", Op: "Transaction Revenue", Asset: "USDT", Delta: 41},
		// две одинаковые сделки в одну секунду — обе настоящие
		{T: 3000, Account: "Spot", Op: "Transaction Buy", Asset: "ETH", Delta: 1},
		{T: 3000, Account: "Spot", Op: "Transaction Spend", Asset: "USDT", Delta: -10},
		{T: 3000, Account: "Spot", Op: "Transaction Buy", Asset: "ETH", Delta: 1},
		{T: 3000, Account: "Spot", Op: "Transaction Spend", Asset: "USDT", Delta: -10},
		// другой кошелёк не берём
		{T: 4000, Account: "Strategy", Op: "Transaction Buy", Asset: "SOL", Delta: 1},
		{T: 4000, Account: "Strategy", Op: "Transaction Spend", Asset: "USDT", Delta: -5},
	}
	got := Trades(rows, "Spot")
	if len(got) != 4 {
		t.Fatalf("ждали 4 сделки Spot, есть %d: %+v", len(got), got)
	}
	ids := map[string]bool{}
	for _, x := range got {
		if ids[x.ID] {
			t.Fatalf("id не уникален: %s", x.ID)
		}
		ids[x.ID] = true
		if x.Wallet != "Spot" || x.Qty <= 0 || x.QuoteQty <= 0 {
			t.Fatalf("%+v", x)
		}
	}
	last := got[len(got)-1]
	if last.Side != "buy" || last.Asset != "BTC" || last.Quote != "USDT" || last.QuoteQty != 40 {
		t.Fatalf("первая сделка: %+v", last)
	}
	if got[2].Side != "sell" || got[2].QuoteQty != 41 {
		t.Fatalf("продажа: %+v", got[2])
	}
}

package binance

import "testing"

func TestSwapsFromJournal(t *testing.T) {
	const s = 1_000
	rows := []Row{
		{T: 1 * s, Account: "Spot", Op: "Binance Convert", Asset: "SOL", Delta: -0.000825},
		{T: 1 * s, Account: "Spot", Op: "Binance Convert", Asset: "BNB", Delta: 0.00010928},
		{T: 2 * s, Account: "Spot", Op: "Small Assets Exchange BNB", Asset: "USDT", Delta: -0.21813612},
		{T: 2 * s, Account: "Spot", Op: "Small Assets Exchange BNB", Asset: "GENIUS", Delta: -3},
		{T: 2 * s, Account: "Spot", Op: "Small Assets Exchange BNB", Asset: "BNB", Delta: 0.00000334},
		{T: 3 * s, Account: "Spot", Op: "WBETH2.0 - Staking", Asset: "ETH", Delta: -0.0124},
		{T: 3 * s, Account: "Spot", Op: "WBETH2.0 - Staking", Asset: "WBETH", Delta: 0.01121637},
		{T: 4 * s, Account: "Spot", Op: "SOL Staking - Purchase", Asset: "SOL", Delta: -0.2},
		{T: 4 * s, Account: "Spot", Op: "SOL Staking - Purchase", Asset: "BNSOL", Delta: 0.17704555},
		{T: 5 * s, Account: "Spot", Op: "Binance Convert", Asset: "USDT", Delta: -1}, // половина без пары — не обмен
		{T: 6 * s, Account: "Spot", Op: "Transaction Buy", Asset: "BTC", Delta: 0.001},
		{T: 6 * s, Account: "Spot", Op: "Transaction Spend", Asset: "USDT", Delta: -60}, // сделка — не обмен, она в Trades
	}
	got := Swaps(rows)
	if len(got) != 4 {
		t.Fatalf("обменов %d, ждали 4: %+v", len(got), got)
	}
	// новые сверху: SOL→BNSOL, ETH→WBETH, пыль→BNB, Convert
	want := []struct {
		kind    string
		out, in string
		nOut    int
	}{{"stake", "SOL", "BNSOL", 1}, {"stake", "ETH", "WBETH", 1}, {"dust", "USDT", "BNB", 2}, {"convert", "SOL", "BNB", 1}}
	for i, w := range want {
		g := got[i]
		if g.Kind != w.kind || g.Out[0].Asset != w.out || g.In[0].Asset != w.in || len(g.Out) != w.nOut || g.Wallet != "Spot" {
			t.Fatalf("обмен %d: %+v, ждали %+v", i, g, w)
		}
	}
	if got[0].Out[0].Qty != 0.2 || got[0].In[0].Qty != 0.17704555 {
		t.Fatalf("количества: %+v", got[0])
	}
	ids := map[string]bool{}
	for _, g := range got {
		if g.ID == "" || ids[g.ID] {
			t.Fatalf("id пустой или повторяется: %q", g.ID)
		}
		ids[g.ID] = true
	}
}

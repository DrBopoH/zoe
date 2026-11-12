package binance

import (
	"fmt"
	"sort"
)

// Trade — сделка внутри кошелька: купили или продали монету за другую. Капитал не меняет (деньги ↔ монеты, комиссия — отдельная строка журнала).
type Trade struct {
	ID       string  `json:"id"`
	T        int64   `json:"t"` // мс
	Wallet   string  `json:"wallet"`
	Side     string  `json:"side"` // buy | sell
	Asset    string  `json:"asset"`
	Qty      float64 `json:"qty"`
	Quote    string  `json:"quote"` // чем заплатили / что получили
	QuoteQty float64 `json:"quote_qty"`
}

// В журнале сделка — две строки одной секунды: «Transaction Buy» (+монета) и «Transaction Spend» (−чем платили),
// «Transaction Sold» (−монета) и «Transaction Revenue» (+что получили).
type tradeKey struct {
	t      int64
	wallet string
}

// Trades — сделки кошельков из списка. Несколько сделок в одну секунду сопоставляются по порядку строк в журнале.
func Trades(rows []Row, wallets ...string) []Trade {
	want := map[string]bool{}
	for _, w := range wallets {
		want[w] = true
	}
	type legs struct{ buy, spend, sold, revenue []Row }
	by := map[tradeKey]*legs{}
	var order []tradeKey
	for _, r := range rows {
		if !want[r.Account] || r.Delta == 0 {
			continue
		}
		k := tradeKey{r.T, r.Account}
		l := by[k]
		if l == nil {
			l = &legs{}
			by[k] = l
			order = append(order, k)
		}
		switch r.Op {
		case "Transaction Buy":
			l.buy = append(l.buy, r)
		case "Transaction Spend":
			l.spend = append(l.spend, r)
		case "Transaction Sold":
			l.sold = append(l.sold, r)
		case "Transaction Revenue":
			l.revenue = append(l.revenue, r)
		}
	}
	var out []Trade
	seen := map[string]int{}
	add := func(k tradeKey, side string, coin, pay Row) {
		qty, quoteQty := coin.Delta, pay.Delta
		if qty < 0 {
			qty = -qty
		}
		if quoteQty < 0 {
			quoteQty = -quoteQty
		}
		base := fmt.Sprintf("tr|%d|%s|%s|%s|%.12g|%s|%.12g", k.t, k.wallet, side, coin.Asset, qty, pay.Asset, quoteQty)
		seen[base]++
		out = append(out, Trade{ID: fmt.Sprintf("%s|%d", base, seen[base]), T: k.t, Wallet: k.wallet, Side: side,
			Asset: coin.Asset, Qty: qty, Quote: pay.Asset, QuoteQty: quoteQty})
	}
	for _, k := range order {
		l := by[k]
		for i := 0; i < len(l.buy) && i < len(l.spend); i++ {
			add(k, "buy", l.buy[i], l.spend[i])
		}
		for i := 0; i < len(l.sold) && i < len(l.revenue); i++ {
			add(k, "sell", l.sold[i], l.revenue[i])
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T > out[j].T })
	return out
}

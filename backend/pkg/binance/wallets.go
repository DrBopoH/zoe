package binance

import (
	"math"
	"sort"
)

// Позиции по кошелькам для карточки счёта: кошелёк → монета, количество, оценка в ₴ и «результат»:
//   Spot/Strategy — прибыль/убыток по последним покупкам относительно текущей цены (delta_kind = pnl);
//   Earn          — последнее начисление в ₴ (delta_kind = last).
// Пустые кошельки не попадают в список.

type WalletItem struct {
	Asset     string   `json:"asset"`
	Qty       float64  `json:"qty"`
	ValueUAH  float64  `json:"value_uah"`
	Priced    bool     `json:"priced"` // false — цены монеты нет: оценка и результат не считаются
	Delta     *float64 `json:"delta,omitempty"`
	DeltaKind string   `json:"delta_kind,omitempty"`
}

type WalletLine struct {
	Wallet string       `json:"wallet"`
	Items  []WalletItem `json:"items"`
}

// WalletLines — позиции по кошелькам в порядке Spot, Funding, Earn, Strategy, Futures.
func WalletLines(h Holdings, rows []Row, prices map[string]float64) []WalletLine {
	usdtUAH := prices["USDTUAH"]
	priceUAH := func(asset string) (float64, bool) {
		if asset == "UAH" {
			return 1, true
		}
		if usdtUAH <= 0 {
			return 0, false
		}
		u, ok := priceUSDT(asset, prices)
		return u * usdtUAH, ok
	}
	trades := Trades(rows, WalletSpot, WalletStrategy) // новые сверху

	// результат по последним покупкам: берём свежие покупки, пока не покроем текущее количество
	pnl := func(wallet, asset string, qty, price float64) *float64 {
		if stable[asset] || asset == "UAH" {
			return nil
		}
		rest, cost := qty, 0.0
		for _, t := range trades {
			if rest <= 1e-12 {
				break
			}
			if t.Wallet != wallet || t.Side != "buy" || t.Asset != asset || t.Qty <= 0 {
				continue
			}
			q, ok := priceUSDT(t.Quote, prices)
			if !ok {
				continue
			}
			take := math.Min(rest, t.Qty)
			cost += t.QuoteQty * take / t.Qty * q
			rest -= take
		}
		covered := qty - rest
		if covered <= 1e-12 {
			return nil
		}
		v := round2(covered*price - cost*usdtUAH)
		return &v
	}
	// последнее начисление Earn (проценты/награды) по монете, в ₴
	lastAccrual := func(asset string, price float64) *float64 {
		for i := len(rows) - 1; i >= 0; i-- {
			r := rows[i]
			if r.Asset == asset && r.Delta > 0 && InPortfolio(r) && Kind(r) == KindDividend {
				v := round2(r.Delta * price)
				return &v
			}
		}
		return nil
	}

	pick := []struct {
		name string
		get  func(*Wallets) float64
	}{
		{WalletSpot, func(w *Wallets) float64 { return w.Spot }},
		{WalletFunding, func(w *Wallets) float64 { return w.Funding }},
		{WalletEarn, func(w *Wallets) float64 { return w.Earn }},
		{WalletStrategy, func(w *Wallets) float64 { return w.Strategy }},
		{WalletFutures, func(w *Wallets) float64 { return w.Futures }},
	}
	var out []WalletLine
	for _, p := range pick {
		var items []WalletItem
		for asset, w := range h {
			qty := p.get(w)
			if qty < dust { // нули и отрицательные хвосты округления (выкуп из Earn больше подписки на копейки)
				continue
			}
			it := WalletItem{Asset: asset, Qty: qty}
			if price, ok := priceUAH(asset); ok {
				it.Priced, it.ValueUAH = true, round2(qty*price)
				switch p.name {
				case WalletSpot, WalletStrategy:
					if d := pnl(p.name, asset, qty, price); d != nil {
						it.Delta, it.DeltaKind = d, "pnl"
					}
				case WalletEarn:
					if d := lastAccrual(asset, price); d != nil {
						it.Delta, it.DeltaKind = d, "last"
					}
				}
			}
			items = append(items, it)
		}
		if len(items) == 0 {
			continue
		}
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].ValueUAH != items[j].ValueUAH {
				return items[i].ValueUAH > items[j].ValueUAH
			}
			return items[i].Asset < items[j].Asset
		})
		out = append(out, WalletLine{Wallet: p.name, Items: items})
	}
	return out
}

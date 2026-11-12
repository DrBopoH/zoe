package binance

import (
	"sort"
	"strings"
)

// Wallets — остатки монеты по кошелькам.
type Wallets struct {
	Spot, Funding, Earn float64
}

func (w Wallets) Total() float64 { return w.Spot + w.Funding + w.Earn }

// Holdings — всё, что лежит на Binance: монета → кошельки.
type Holdings map[string]*Wallets

func (h Holdings) add(asset string, v float64, set func(*Wallets, float64)) {
	if v == 0 {
		return
	}
	// LD* — «зеркало» Simple Earn в споте: оценивается по настоящей монете, а сами остатки приходят из Earn-эндпоинтов
	if strings.HasPrefix(asset, "LD") && len(asset) > 2 {
		return
	}
	w := h[asset]
	if w == nil {
		w = &Wallets{}
		h[asset] = w
	}
	set(w, v)
}

func (h Holdings) AddSpot(m map[string]float64) {
	for a, v := range m {
		h.add(a, v, func(w *Wallets, x float64) { w.Spot += x })
	}
}
func (h Holdings) AddFunding(m map[string]float64) {
	for a, v := range m {
		h.add(a, v, func(w *Wallets, x float64) { w.Funding += x })
	}
}
func (h Holdings) AddEarn(m map[string]float64) {
	for a, v := range m {
		h.add(a, v, func(w *Wallets, x float64) { w.Earn += x })
	}
}

// Item — монета с оценкой в гривнах.
type Item struct {
	Asset    string  `json:"asset"`
	Amount   float64 `json:"amount"`
	Spot     float64 `json:"spot"`
	Funding  float64 `json:"funding"`
	Earn     float64 `json:"earn"`
	PriceUAH float64 `json:"price_uah"`
	ValueUAH float64 `json:"value_uah"`
}

type Valuation struct {
	TotalUAH float64  `json:"total_uah"`
	USDTUAH  float64  `json:"usdt_uah"`
	Items    []Item   `json:"items"`
	Unpriced []string `json:"unpriced,omitempty"` // монеты, для которых не нашлось цены (в сумму не входят)
}

// stable — монеты, которые считаем равными 1 USDT (если нет прямой пары).
var stable = map[string]bool{"USDT": true, "USDC": true, "FDUSD": true, "BUSD": true, "TUSD": true, "USDP": true, "DAI": true}

// priceUSDT — цена монеты в USDT: прямая пара XXXUSDT, иначе через BTC/ETH/BNB.
func priceUSDT(asset string, p map[string]float64) (float64, bool) {
	if asset == "USDT" {
		return 1, true
	}
	if v := p[asset+"USDT"]; v > 0 {
		return v, true
	}
	if stable[asset] {
		return 1, true
	}
	for _, mid := range []string{"BTC", "ETH", "BNB"} {
		if a, b := p[asset+mid], p[mid+"USDT"]; a > 0 && b > 0 {
			return a * b, true
		}
	}
	return 0, false
}

// Value оценивает остатки в гривнах: монета → USDT → ₴ по паре USDTUAH. Монета UAH на счёте Binance считается как есть.
func Value(h Holdings, prices map[string]float64) (Valuation, bool) {
	var v Valuation
	v.USDTUAH = prices["USDTUAH"]
	if v.USDTUAH <= 0 {
		return v, false // без курса гривны оценивать нечем
	}
	for asset, w := range h {
		total := w.Total()
		if total == 0 {
			continue
		}
		it := Item{Asset: asset, Amount: total, Spot: w.Spot, Funding: w.Funding, Earn: w.Earn}
		switch {
		case asset == "UAH":
			it.PriceUAH = 1
		default:
			usdt, ok := priceUSDT(asset, prices)
			if !ok {
				v.Unpriced = append(v.Unpriced, asset)
				v.Items = append(v.Items, it)
				continue
			}
			it.PriceUAH = usdt * v.USDTUAH
		}
		it.ValueUAH = round2(total * it.PriceUAH)
		v.TotalUAH += it.ValueUAH
		v.Items = append(v.Items, it)
	}
	v.TotalUAH = round2(v.TotalUAH)
	sort.Strings(v.Unpriced)
	sort.Slice(v.Items, func(i, j int) bool {
		if v.Items[i].ValueUAH != v.Items[j].ValueUAH {
			return v.Items[i].ValueUAH > v.Items[j].ValueUAH
		}
		return v.Items[i].Asset < v.Items[j].Asset
	})
	return v, true
}

func round2(x float64) float64 {
	if x < 0 {
		return -float64(int64(-x*100+0.5)) / 100
	}
	return float64(int64(x*100+0.5)) / 100
}

// Fetch собирает остатки по всем кошелькам. Spot обязателен; Funding и Earn — по возможности
// (если у ключа нет права или кошелька, пишем предупреждение и идём дальше).
func (c *Client) Fetch() (Holdings, []string, error) {
	var warns []string
	_ = c.SyncTime()
	h := Holdings{}
	spot, err := c.Spot()
	if err != nil {
		return nil, nil, err
	}
	h.AddSpot(spot)
	if m, err := c.Funding(); err != nil {
		warns = append(warns, "Funding: "+err.Error())
	} else {
		h.AddFunding(m)
	}
	if m, err := c.EarnFlexible(); err != nil {
		warns = append(warns, "Earn (flexible): "+err.Error())
	} else {
		h.AddEarn(m)
	}
	if m, err := c.EarnLocked(); err != nil {
		warns = append(warns, "Earn (locked): "+err.Error())
	} else {
		h.AddEarn(m)
	}
	return h, warns, nil
}

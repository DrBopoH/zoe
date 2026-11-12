package binance

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// dust — остаток меньше этого считается нулём (сумма сотен строк выписки даёт шум вроде 1e-17).
const dust = 1e-9

// Wallets — остатки монеты по кошелькам.
type Wallets struct {
	Spot, Funding, Earn, Strategy, Futures float64
}

func (w Wallets) Total() float64 { return w.Spot + w.Funding + w.Earn + w.Strategy + w.Futures }

// Holdings — всё, что лежит на Binance: монета → кошельки.
type Holdings map[string]*Wallets

func (h Holdings) add(asset string, v float64, set func(*Wallets, float64)) {
	if math.Abs(v) < dust { // следы округления сумм в выписке
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
func (h Holdings) AddStrategy(m map[string]float64) {
	for a, v := range m {
		h.add(a, v, func(w *Wallets, x float64) { w.Strategy += x })
	}
}
func (h Holdings) AddFutures(m map[string]float64) {
	for a, v := range m {
		h.add(a, v, func(w *Wallets, x float64) { w.Futures += x })
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
	Strategy float64 `json:"strategy"`
	Futures  float64 `json:"futures"`
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
		it := Item{Asset: asset, Amount: total, Spot: w.Spot, Funding: w.Funding, Earn: w.Earn, Strategy: w.Strategy, Futures: w.Futures}
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

// minuteClose — цена закрытия последней свечи пары не позже этой минуты (на неликвидной паре минута без сделок = цена
// последней сделки, как у минутной сетки); если до неё свечей нет — первая после. Результат запоминается.
func (c *Client) minuteClose(symbol string, tMs int64) (float64, error) {
	t := tMs / minute * minute
	key := symbol + "|" + strconv.FormatInt(t, 10)
	c.mu.Lock()
	if v, ok := c.px[key]; ok {
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()
	var r [][]json.RawMessage
	for _, q := range []string{"endTime", "startTime"} { // сначала последняя сделка до минуты, потом первая после
		body, err := c.do("/api/v3/klines", url.Values{"symbol": {symbol}, "interval": {"1m"}, q: {strconv.FormatInt(t, 10)}, "limit": {"1"}})
		if err != nil {
			return 0, err
		}
		r = nil
		if err := json.Unmarshal(body, &r); err != nil {
			return 0, err
		}
		if len(r) > 0 && len(r[0]) >= 5 {
			break
		}
	}
	if len(r) == 0 || len(r[0]) < 5 {
		return 0, fmt.Errorf("нет цены %s на %s", symbol, time.UnixMilli(t).Format("2006-01-02 15:04"))
	}
	var s string
	if err := json.Unmarshal(r[0][4], &s); err != nil {
		return 0, err
	}
	v := num(s)
	c.mu.Lock()
	if c.px == nil {
		c.px = map[string]float64{}
	}
	c.px[key] = v
	c.mu.Unlock()
	return v, nil
}

// ValueUAH — сколько стоило qty монеты в гривнах в момент tMs (по минутным свечам: монета → USDT → ₴).
func (c *Client) ValueUAH(asset string, qty float64, tMs int64) (float64, error) {
	if asset == "UAH" {
		return qty, nil
	}
	usdt := 1.0
	if !stable[asset] {
		syms, err := c.Symbols()
		if err != nil {
			return 0, err
		}
		usdt = 0
		if syms[asset+"USDT"].Base != "" {
			if usdt, err = c.minuteClose(asset+"USDT", tMs); err != nil {
				return 0, err
			}
		} else {
			for _, m := range []string{"BTC", "ETH", "BNB"} {
				if syms[asset+m].Base == "" {
					continue
				}
				a, err := c.minuteClose(asset+m, tMs)
				if err != nil {
					return 0, err
				}
				b, err := c.minuteClose(m+"USDT", tMs)
				if err != nil {
					return 0, err
				}
				usdt = a * b
				break
			}
		}
		if usdt <= 0 {
			return 0, fmt.Errorf("нет пары для оценки %s", asset)
		}
	}
	rate, err := c.minuteClose("USDTUAH", tMs)
	if err != nil {
		return 0, err
	}
	return math.Round(qty*usdt*rate*100) / 100, nil
}

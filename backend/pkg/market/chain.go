package market

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Крипто-контур для цепочки «банк → P2P USDT → торговля → вывод». Форматы ответов подтверждены зондом 16.11.2026 (probe9):
// P2P Binance (POST adv/search), фьючерсы Binance (fundingRate, openInterestHist), DefiLlama (stablecoincharts), mempool.space, TronGrid, публичный RPC Ethereum, CoinGecko.
// Мгновенные показатели (P2P, комиссии сетей, доминация) — снимки на дату опроса: история копится в кеше от обновления к обновлению.
// Каждая часть независима: сбой одного источника не трогает остальные, ошибка — в Result.Error.
const (
	SourceChain = "chain"

	SerP2PAsk      = "p2p_ask"      // P2P: за сколько гривен можно купить 1 USDT (типичная лучшая цена, медиана первых пяти рабочих объявлений)
	SerP2PBid      = "p2p_bid"      // P2P: сколько гривен дают за 1 USDT при продаже
	SerFundBTC     = "fund_btc"     // фандинг бессрочного фьючерса BTCUSDT, % годовых (среднее за сутки × 3 × 365)
	SerFundETH     = "fund_eth"     // то же для ETHUSDT
	SerOIBTC       = "oi_btc"       // открытый интерес BTCUSDT, $ млрд (Binance хранит только последние 30 дней)
	SerOIETH       = "oi_eth"       // то же для ETHUSDT
	SerStableUSDT  = "stable_usdt"  // выпущено USDT, $ млрд (DefiLlama, неделя)
	SerStableUSDC  = "stable_usdc"  // выпущено USDC, $ млрд
	SerFeeBTCSat   = "fee_btc_sat"  // медианная комиссия блоков Bitcoin, сат/vB, среднее за сутки (mempool.space, 3 месяца)
	SerFeeBTCUSD   = "fee_btc_usd"  // перевод Bitcoin (140 vB, «полчаса»), $ — снимок
	SerFeeTronUSD  = "fee_tron_usd" // перевод USDT по сети TRON (энергия 65 000 + трафик), $ — снимок
	SerFeeEthUSD   = "fee_eth_usd"  // перевод USDT по сети Ethereum (65 000 газа), $ — снимок
	SerDomBTC      = "dom_btc"      // доля Bitcoin в капитализации крипторынка, % — снимок
	SerMcapTotal   = "mcap_total"   // капитализация крипторынка, $ трлн — снимок
	p2pTicketUAH   = 10000.0        // «рабочее» объявление: на сумму 10 000 ₴ хватает остатка и она вписывается в лимиты
	p2pTopN        = 5              // сколько первых рабочих объявлений усреднять (медиана)
	btcTxVBytes    = 140.0          // типичный перевод: 1 вход, 2 выхода, segwit
	tronEnergyUSDT = 65000.0        // энергия перевода USDT, если у получателя уже есть USDT
	tronBandwidth  = 345.0          // трафик перевода, байт
	ethUSDTGas     = 65000.0        // газ перевода ERC-20
)

var chainSeries = []string{
	SerP2PAsk, SerP2PBid, SerFundBTC, SerFundETH, SerOIBTC, SerOIETH, SerStableUSDT, SerStableUSDC,
	SerFeeBTCSat, SerFeeBTCUSD, SerFeeTronUSD, SerFeeEthUSD, SerDomBTC, SerMcapTotal,
}

// SnapshotSeries — ряды-снимки: источник отдаёт только «сейчас» (истории нет). Они копятся сами: в кеше — по дням (хвостом, раньшие дни остаются),
// и в macro_ticks — по часам (db.SaveMacroTicks). Остальные ряды контура приходят с историей и заменяются целиком.
var SnapshotSeries = []string{SerP2PAsk, SerP2PBid, SerFeeBTCUSD, SerFeeTronUSD, SerFeeEthUSD, SerDomBTC, SerMcapTotal}

// IsSnapshot — ряд из SnapshotSeries.
func IsSnapshot(id string) bool {
	for _, s := range SnapshotSeries {
		if s == id {
			return true
		}
	}
	return false
}

// TickEvery — как часто сервер сам снимает снимки; точка привязана к сетке (началу часа), а не к моменту запуска сервера.
const TickEvery = time.Hour

// NextTick — сколько ждать до следующего снимка по сетке every (кратной суткам). last — время последнего сохранённого снимка (нулевое — не было).
// Если за текущий слот сетки снимка ещё нет (сервер был выключен или только что запущен), ждать не нужно: рестарт не сдвигает и не теряет слот.
func NextTick(now, last time.Time, every time.Duration) time.Duration {
	if every <= 0 {
		return 0
	}
	slot := now.UTC().Truncate(every)
	if last.Before(slot) {
		return 0
	}
	return slot.Add(every).Sub(now)
}

// Chain — все показатели крипто-контура.
func (c *Client) Chain(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceChain}
	var out []SeriesPoint
	var errs []string
	part := func(pct int, note string, f func() ([]SeriesPoint, error)) {
		rep.report(pct, note)
		pts, err := f()
		if err != nil {
			errs = append(errs, err.Error())
		}
		out = append(out, pts...)
	}
	today := c.now().UTC().Format("2006-01-02")
	part(5, "P2P USDT/UAH", func() ([]SeriesPoint, error) { return c.p2pPrices(ctx, today) })
	part(20, "фандинг BTC", func() ([]SeriesPoint, error) { return c.funding(ctx, "BTCUSDT", SerFundBTC) })
	part(30, "фандинг ETH", func() ([]SeriesPoint, error) { return c.funding(ctx, "ETHUSDT", SerFundETH) })
	part(40, "открытый интерес", func() ([]SeriesPoint, error) {
		a, e1 := c.openInterest(ctx, "BTCUSDT", SerOIBTC)
		b, e2 := c.openInterest(ctx, "ETHUSDT", SerOIETH)
		return append(a, b...), firstErr(e1, e2)
	})
	part(55, "стейблкоины", func() ([]SeriesPoint, error) {
		a, e1 := c.stableSupply(ctx, 1, SerStableUSDT)
		b, e2 := c.stableSupply(ctx, 2, SerStableUSDC)
		return append(a, b...), firstErr(e1, e2)
	})
	part(70, "комиссии сетей", func() ([]SeriesPoint, error) { return c.networkFees(ctx, today) })
	part(90, "доминация и капитализация", func() ([]SeriesPoint, error) { return c.global(ctx, today) })
	res.Fetched, res.Error = len(out), strings.Join(errs, "; ")
	res.Tail = SnapshotSeries // снимки докладываются к кешу, а не заменяют его: так копится история
	return out, res
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func round(v float64, digits int) float64 {
	k := math.Pow(10, float64(digits))
	return math.Round(v*k) / k
}

func f64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n
	}
	return 0
}

// ───────────────────────── P2P ─────────────────────────

type p2pAdv struct {
	Adv struct {
		Price    string `json:"price"`
		Tradable string `json:"tradableQuantity"`
		Min      string `json:"minSingleTransAmount"`
		Max      string `json:"maxSingleTransAmount"`
	} `json:"adv"`
}

// p2pPrice — типичная лучшая цена стороны: медиана первых p2pTopN рабочих объявлений. Объявление рабочее, если остатка хватает на 10 000 ₴ и сумма вписывается в его лимиты:
// единичное объявление с крошечным остатком не должно задавать «рынок». side — BUY (покупаем USDT, платим гривной) или SELL.
func (c *Client) p2pPrice(ctx context.Context, side string) (float64, error) {
	payload := fmt.Sprintf(`{"fiat":"UAH","page":1,"rows":20,"tradeType":%q,"asset":"USDT","countries":[],"proMerchantAds":false,"shieldMerchantAds":false,"publisherType":null,"payTypes":[],"classifies":["mass","profession"]}`, side)
	body, err := c.postJSON(ctx, strings.TrimRight(c.P2PBase, "/")+"/bapi/c2c/v2/friendly/c2c/adv/search", []byte(payload))
	if err != nil {
		return 0, fmt.Errorf("P2P %s: %w", side, err)
	}
	var r struct {
		Data []p2pAdv `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, fmt.Errorf("P2P %s: %w", side, err)
	}
	var prices []float64
	for _, a := range r.Data {
		price, qty := f64(a.Adv.Price), f64(a.Adv.Tradable)
		lo, hi := f64(a.Adv.Min), f64(a.Adv.Max)
		if price <= 0 || qty*price < p2pTicketUAH || lo > p2pTicketUAH || (hi > 0 && hi < p2pTicketUAH) {
			continue
		}
		prices = append(prices, price)
		if len(prices) == p2pTopN {
			break
		}
	}
	if len(prices) == 0 {
		return 0, fmt.Errorf("P2P %s: нет рабочих объявлений", side)
	}
	sort.Float64s(prices)
	return prices[len(prices)/2], nil
}

func (c *Client) p2pPrices(ctx context.Context, today string) ([]SeriesPoint, error) {
	if c.P2PBase == "" {
		return nil, nil
	}
	var out []SeriesPoint
	var errs []error
	for _, s := range []struct{ side, series string }{{"BUY", SerP2PAsk}, {"SELL", SerP2PBid}} {
		v, err := c.p2pPrice(ctx, s.side)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, SeriesPoint{Series: s.series, Date: today, Value: round(v, 2)})
	}
	return out, firstErr(errs...)
}

// ───────────────────────── фьючерсы ─────────────────────────

// funding — история фандинга (каждые 8 часов, до ~1000 записей ≈ 11 месяцев), по суткам: % годовых.
func (c *Client) funding(ctx context.Context, symbol, series string) ([]SeriesPoint, error) {
	if c.FuturesBase == "" {
		return nil, nil
	}
	body, err := c.get(ctx, strings.TrimRight(c.FuturesBase, "/")+"/fapi/v1/fundingRate?symbol="+symbol+"&limit=1000")
	if err != nil {
		return nil, fmt.Errorf("фандинг %s: %w", symbol, err)
	}
	var rows []struct {
		Time int64  `json:"fundingTime"`
		Rate string `json:"fundingRate"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("фандинг %s: %w", symbol, err)
	}
	sum, n := map[string]float64{}, map[string]int{}
	for _, r := range rows {
		v, err := strconv.ParseFloat(r.Rate, 64)
		if err != nil || r.Time <= 0 {
			continue
		}
		d := time.UnixMilli(r.Time).UTC().Format("2006-01-02")
		sum[d] += v
		n[d]++
	}
	return dailySeries(series, sum, n, func(mean float64) float64 { return round(mean*3*365*100, 2) }), nil
}

func dailySeries(series string, sum map[string]float64, n map[string]int, conv func(float64) float64) []SeriesPoint {
	dates := make([]string, 0, len(sum))
	for d := range sum {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	out := make([]SeriesPoint, 0, len(dates))
	for _, d := range dates {
		out = append(out, SeriesPoint{Series: series, Date: d, Value: conv(sum[d] / float64(n[d]))})
	}
	return out
}

// openInterest — открытый интерес в долларах по суткам, млрд (Binance отдаёт только последние 30 дней).
func (c *Client) openInterest(ctx context.Context, symbol, series string) ([]SeriesPoint, error) {
	if c.FuturesBase == "" {
		return nil, nil
	}
	body, err := c.get(ctx, strings.TrimRight(c.FuturesBase, "/")+"/futures/data/openInterestHist?symbol="+symbol+"&period=1d&limit=30")
	if err != nil {
		return nil, fmt.Errorf("открытый интерес %s: %w", symbol, err)
	}
	var rows []struct {
		Value string `json:"sumOpenInterestValue"`
		Time  int64  `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("открытый интерес %s: %w", symbol, err)
	}
	var out []SeriesPoint
	for _, r := range rows {
		v, err := strconv.ParseFloat(r.Value, 64)
		if err != nil || v <= 0 || r.Time <= 0 {
			continue
		}
		out = append(out, SeriesPoint{Series: series, Date: time.UnixMilli(r.Time).UTC().Format("2006-01-02"), Value: round(v/1e9, 2)})
	}
	return out, nil
}

// ───────────────────────── стейблкоины ─────────────────────────

// stableSupply — выпущенное количество стейблкоина (id DefiLlama: 1 — USDT, 2 — USDC), $ млрд, последнее значение недели.
func (c *Client) stableSupply(ctx context.Context, id int, series string) ([]SeriesPoint, error) {
	if c.LlamaBase == "" {
		return nil, nil
	}
	body, err := c.get(ctx, fmt.Sprintf("%s/stablecoincharts/all?stablecoin=%d", strings.TrimRight(c.LlamaBase, "/"), id))
	if err != nil {
		return nil, fmt.Errorf("стейблкоин %s: %w", series, err)
	}
	var rows []struct {
		Date  string `json:"date"`
		Total struct {
			USD float64 `json:"peggedUSD"`
		} `json:"totalCirculatingUSD"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("стейблкоин %s: %w", series, err)
	}
	var pts []SeriesPoint
	for _, r := range rows {
		sec, err := strconv.ParseInt(r.Date, 10, 64)
		if err != nil || r.Total.USD <= 0 {
			continue
		}
		pts = append(pts, SeriesPoint{Series: series, Date: time.Unix(sec, 0).UTC().Format("2006-01-02"), Value: round(r.Total.USD/1e9, 3)})
	}
	return weekly(pts), nil
}

// ───────────────────────── комиссии сетей ─────────────────────────

func (c *Client) spot(ctx context.Context, symbol string) (float64, error) {
	body, err := c.get(ctx, strings.TrimRight(c.BinanceBase, "/")+"/api/v3/ticker/price?symbol="+symbol)
	if err != nil {
		return 0, fmt.Errorf("цена %s: %w", symbol, err)
	}
	var r struct {
		Price string `json:"price"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, fmt.Errorf("цена %s: %w", symbol, err)
	}
	v, _ := strconv.ParseFloat(r.Price, 64)
	if v <= 0 {
		return 0, fmt.Errorf("цена %s: нет значения", symbol)
	}
	return v, nil
}

// networkFees — комиссии Bitcoin (история блоков за 3 месяца и снимок «полчаса» в долларах), TRON и Ethereum (снимки в долларах).
func (c *Client) networkFees(ctx context.Context, today string) ([]SeriesPoint, error) {
	var out []SeriesPoint
	var errs []error
	if c.MempoolBase != "" {
		pts, err := c.btcFees(ctx, today)
		out = append(out, pts...)
		errs = append(errs, err)
	}
	if c.TronBase != "" {
		pts, err := c.tronFee(ctx, today)
		out = append(out, pts...)
		errs = append(errs, err)
	}
	if c.EthRPCBase != "" {
		pts, err := c.ethFee(ctx, today)
		out = append(out, pts...)
		errs = append(errs, err)
	}
	return out, firstErr(errs...)
}

func (c *Client) btcFees(ctx context.Context, today string) ([]SeriesPoint, error) {
	base := strings.TrimRight(c.MempoolBase, "/")
	var out []SeriesPoint
	var errs []error
	if body, err := c.get(ctx, base+"/api/v1/mining/blocks/fee-rates/3m"); err != nil {
		errs = append(errs, fmt.Errorf("комиссия BTC (история): %w", err))
	} else {
		var rows []struct {
			Time int64   `json:"timestamp"`
			P50  float64 `json:"avgFee_50"`
		}
		if err := json.Unmarshal(body, &rows); err != nil {
			errs = append(errs, fmt.Errorf("комиссия BTC (история): %w", err))
		}
		sum, n := map[string]float64{}, map[string]int{}
		for _, r := range rows {
			if r.Time <= 0 || r.P50 < 0 {
				continue
			}
			d := time.Unix(r.Time, 0).UTC().Format("2006-01-02")
			sum[d] += r.P50
			n[d]++
		}
		out = append(out, dailySeries(SerFeeBTCSat, sum, n, func(m float64) float64 { return round(m, 2) })...)
	}
	body, err := c.get(ctx, base+"/api/v1/fees/recommended")
	if err != nil {
		return out, firstErr(append(errs, fmt.Errorf("комиссия BTC: %w", err))...)
	}
	var rec struct {
		HalfHour float64 `json:"halfHourFee"`
	}
	if err := json.Unmarshal(body, &rec); err != nil || rec.HalfHour <= 0 {
		return out, firstErr(append(errs, fmt.Errorf("комиссия BTC: нет значения"))...)
	}
	price, err := c.spot(ctx, "BTCUSDT")
	if err != nil {
		return out, firstErr(append(errs, err)...)
	}
	out = append(out, SeriesPoint{Series: SerFeeBTCUSD, Date: today, Value: round(rec.HalfHour*btcTxVBytes/1e8*price, 4)})
	return out, firstErr(errs...)
}

// tronFee — перевод USDT в TRON: энергия и трафик по текущим параметрам сети (цены в sun, 1 TRX = 1 000 000 sun) на цену TRX.
// Считается без «замороженных» ресурсов, то есть худший случай — сжигание TRX.
func (c *Client) tronFee(ctx context.Context, today string) ([]SeriesPoint, error) {
	body, err := c.get(ctx, strings.TrimRight(c.TronBase, "/")+"/wallet/getchainparameters")
	if err != nil {
		return nil, fmt.Errorf("комиссия TRON: %w", err)
	}
	var r struct {
		Params []struct {
			Key   string  `json:"key"`
			Value float64 `json:"value"`
		} `json:"chainParameter"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("комиссия TRON: %w", err)
	}
	var energy, bandwidth float64
	for _, p := range r.Params {
		switch p.Key {
		case "getEnergyFee":
			energy = p.Value
		case "getTransactionFee":
			bandwidth = p.Value
		}
	}
	if energy <= 0 || bandwidth <= 0 {
		return nil, fmt.Errorf("комиссия TRON: нет getEnergyFee или getTransactionFee")
	}
	trx, err := c.spot(ctx, "TRXUSDT")
	if err != nil {
		return nil, err
	}
	fee := (tronEnergyUSDT*energy + tronBandwidth*bandwidth) / 1e6 * trx
	return []SeriesPoint{{Series: SerFeeTronUSD, Date: today, Value: round(fee, 4)}}, nil
}

// ethFee — перевод USDT в Ethereum: текущая цена газа из публичного RPC на 65 000 газа и цену ETH.
func (c *Client) ethFee(ctx context.Context, today string) ([]SeriesPoint, error) {
	body, err := c.postJSON(ctx, strings.TrimRight(c.EthRPCBase, "/"), []byte(`{"jsonrpc":"2.0","method":"eth_gasPrice","params":[],"id":1}`))
	if err != nil {
		return nil, fmt.Errorf("комиссия ETH: %w", err)
	}
	var r struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("комиссия ETH: %w", err)
	}
	wei, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(r.Result), "0x"), 16, 64)
	if err != nil || wei == 0 {
		return nil, fmt.Errorf("комиссия ETH: нет цены газа")
	}
	eth, err := c.spot(ctx, "ETHUSDT")
	if err != nil {
		return nil, err
	}
	fee := ethUSDTGas * float64(wei) / 1e18 * eth
	return []SeriesPoint{{Series: SerFeeEthUSD, Date: today, Value: round(fee, 4)}}, nil
}

// ───────────────────────── рынок в целом ─────────────────────────

func (c *Client) global(ctx context.Context, today string) ([]SeriesPoint, error) {
	if c.GeckoBase == "" {
		return nil, nil
	}
	body, err := c.get(ctx, strings.TrimRight(c.GeckoBase, "/")+"/api/v3/global")
	if err != nil {
		return nil, fmt.Errorf("CoinGecko: %w", err)
	}
	var r struct {
		Data struct {
			Pct  map[string]float64 `json:"market_cap_percentage"`
			Caps map[string]float64 `json:"total_market_cap"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("CoinGecko: %w", err)
	}
	var out []SeriesPoint
	if v := r.Data.Pct["btc"]; v > 0 {
		out = append(out, SeriesPoint{Series: SerDomBTC, Date: today, Value: round(v, 2)})
	}
	if v := r.Data.Caps["usd"]; v > 0 {
		out = append(out, SeriesPoint{Series: SerMcapTotal, Date: today, Value: round(v/1e12, 3)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("CoinGecko: нет данных")
	}
	return out, nil
}

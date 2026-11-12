package market

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Крипторынок: недельные цены монет с публичных свечей Binance и индекс страха и жадности (alternative.me). Ключей нет.
// Формат ответов — по документации (свечи: массив массивов [время открытия мс, open, high, low, close, …], числа строками;
// индекс: {"data":[{"value":"25","timestamp":"1699833600"}]}); разбор терпимый: число принимается строкой или числом.
// ПЕРЕД ПЕРВЫМ ЗАПУСКОМ сверить живым curl: см. docs/ARCHITECTURE.md (раздел «Крипто»).
const (
	SourceCrypto = "crypto"

	SerCryptoBTC = "crypto_btc" // Bitcoin, USDT за монету, закрытие недели
	SerCryptoETH = "crypto_eth"
	SerCryptoBNB = "crypto_bnb"
	SerCryptoSOL = "crypto_sol"
	SerCryptoFNG = "crypto_fng" // индекс страха и жадности, 0 (крайний страх) … 100 (крайняя жадность), последнее значение недели
)

var cryptoPairs = []struct{ symbol, series string }{
	{"BTCUSDT", SerCryptoBTC}, {"ETHUSDT", SerCryptoETH}, {"BNBUSDT", SerCryptoBNB}, {"SOLUSDT", SerCryptoSOL},
}

// Crypto — недельные цены монет и индекс страха и жадности. Сбой одного запроса не убивает остальные (ошибка — в Result.Error).
func (c *Client) Crypto(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceCrypto}
	var out []SeriesPoint
	var errs []string
	for i, p := range cryptoPairs {
		rep.report(10+i*18, p.symbol)
		pts, sk, err := c.binanceWeekly(ctx, p.symbol, p.series)
		res.Skipped += sk
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		out = append(out, pts...)
	}
	rep.report(85, "индекс страха и жадности")
	fng, sk, err := c.fearGreed(ctx)
	res.Skipped += sk
	if err != nil {
		errs = append(errs, err.Error())
	}
	out = append(out, fng...)
	rep.report(95, "")
	res.Fetched = len(out)
	res.Error = strings.Join(errs, "; ")
	return out, res
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, !math.IsNaN(x)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil && !math.IsNaN(f)
	}
	return 0, false
}

// binanceWeekly — до 1000 недель (≈19 лет: хватает на всю историю) за один запрос; ряд идёт от старых к новым.
func (c *Client) binanceWeekly(ctx context.Context, symbol, series string) ([]SeriesPoint, int, error) {
	body, err := c.get(ctx, strings.TrimRight(c.BinanceBase, "/")+"/api/v3/klines?symbol="+symbol+"&interval=1w&limit=1000")
	if err != nil {
		return nil, 0, fmt.Errorf("Binance %s: %w", symbol, err)
	}
	var rows [][]any
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, 0, fmt.Errorf("Binance %s: %w", symbol, err)
	}
	var out []SeriesPoint
	skipped := 0
	for _, r := range rows {
		if len(r) < 5 {
			skipped++
			continue
		}
		ms, ok1 := asFloat(r[0])
		closePrice, ok2 := asFloat(r[4])
		if !ok1 || !ok2 || closePrice <= 0 {
			skipped++
			continue
		}
		out = append(out, SeriesPoint{Series: series, Date: time.UnixMilli(int64(ms)).UTC().Format("2006-01-02"), Value: closePrice})
	}
	if len(out) == 0 {
		return nil, skipped, fmt.Errorf("Binance %s: пустой ответ", symbol)
	}
	return out, skipped, nil
}

// fearGreed — вся история индекса (limit=0), дальше последнее значение каждой недели.
func (c *Client) fearGreed(ctx context.Context) ([]SeriesPoint, int, error) {
	body, err := c.get(ctx, strings.TrimRight(c.FNGBase, "/")+"/fng/?limit=0&format=json")
	if err != nil {
		return nil, 0, fmt.Errorf("индекс страха и жадности: %w", err)
	}
	var resp struct {
		Data []struct {
			Value     any `json:"value"`
			Timestamp any `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("индекс страха и жадности: %w", err)
	}
	var pts []SeriesPoint
	skipped := 0
	for _, d := range resp.Data {
		v, ok1 := asFloat(d.Value)
		ts, ok2 := asFloat(d.Timestamp)
		if !ok1 || !ok2 {
			skipped++
			continue
		}
		pts = append(pts, SeriesPoint{Series: SerCryptoFNG, Date: time.Unix(int64(ts), 0).UTC().Format("2006-01-02"), Value: v})
	}
	if len(pts) == 0 {
		return nil, skipped, fmt.Errorf("индекс страха и жадности: пустой ответ")
	}
	return weekly(pts), skipped, nil
}

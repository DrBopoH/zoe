package market

import (
	"context"
	"strings"
)

// Фондовый рынок США из FRED (тот же публичный CSV без ключа): три главных индекса, последнее закрытие недели.
// FRED отдаёт S&P 500 и Dow Jones примерно за последние 10 лет (лицензия), NASDAQ Composite — с 1971 года.
const (
	SourceStocks = "stocks"

	SerStockSP500  = "stock_sp500"  // S&P 500 (SP500), пункты
	SerStockNasdaq = "stock_nasdaq" // NASDAQ Composite (NASDAQCOM), пункты
	SerStockDow    = "stock_dow"    // Dow Jones Industrial Average (DJIA), пункты
)

// Stocks — ряды фондовых индексов. Сбой одного индекса не убивает остальные, ошибка — в Result.Error.
func (c *Client) Stocks(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceStocks}
	var out []SeriesPoint
	var errs []string
	for _, f := range []struct {
		fred, series, note string
		pct                int
	}{
		{"SP500", SerStockSP500, "S&P 500", 15},
		{"NASDAQCOM", SerStockNasdaq, "NASDAQ", 50},
		{"DJIA", SerStockDow, "Dow Jones", 80},
	} {
		rep.report(f.pct, f.note)
		obs, sk, err := c.fred(ctx, f.fred)
		res.Skipped += sk
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		out = append(out, weekly(toPoints(f.series, obs))...)
	}
	rep.report(95, "")
	res.Fetched = len(out)
	res.Error = strings.Join(errs, "; ")
	return out, res
}

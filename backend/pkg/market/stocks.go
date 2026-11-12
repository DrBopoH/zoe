package market

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Фондовый рынок США из FRED (тот же публичный CSV без ключа): три главных индекса, последнее закрытие недели.
// FRED отдаёт S&P 500 и Dow Jones примерно за последние 10 лет (лицензия), NASDAQ Composite — с 1971 года.
// Историю S&P 500 до окна FRED добирает месячный ряд Роберта Шиллера (с 1871 года, среднее за месяц) из открытого датасета на GitHub.
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
	var fredSP []SeriesPoint
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
		pts := weekly(toPoints(f.series, obs))
		if f.series == SerStockSP500 {
			fredSP = pts // склеим с историей Шиллера ниже
			continue
		}
		out = append(out, pts...)
	}
	rep.report(88, "S&P 500: история с 1871 года")
	hist, err := c.shillerSP500(ctx)
	if err != nil {
		errs = append(errs, err.Error())
	}
	out = append(out, stitchHistory(hist, fredSP)...)
	rep.report(95, "")
	res.Fetched = len(out)
	res.Error = strings.Join(errs, "; ")
	return out, res
}

// stitchHistory: месячная история + недельный ряд; из истории берутся только точки раньше первой недельной.
func stitchHistory(hist, recent []SeriesPoint) []SeriesPoint {
	if len(recent) == 0 {
		return hist
	}
	first := recent[0].Date
	var out []SeriesPoint
	for _, p := range hist {
		if p.Date < first {
			out = append(out, p)
		}
	}
	return append(out, recent...)
}

// shillerSP500 читает месячный S&P 500 (колонки Date, SP500, …) из датасета datasets/s-and-p-500.
func (c *Client) shillerSP500(ctx context.Context) ([]SeriesPoint, error) {
	body, err := c.get(ctx, strings.TrimRight(c.ShillerBase, "/")+"/datasets/s-and-p-500/main/data/data.csv")
	if err != nil {
		return nil, fmt.Errorf("Шиллер S&P 500: %w", err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(body))).ReadAll()
	if err != nil || len(rows) < 2 || len(rows[0]) < 2 {
		return nil, fmt.Errorf("Шиллер S&P 500: пустой или неверный ответ")
	}
	var out []SeriesPoint
	for _, r := range rows[1:] {
		if len(r) < 2 {
			continue
		}
		d := normDate(r[0])
		v, err := strconv.ParseFloat(strings.TrimSpace(r[1]), 64)
		if d == "" || err != nil || math.IsNaN(v) || v <= 0 {
			continue
		}
		out = append(out, SeriesPoint{Series: SerStockSP500, Date: d, Value: v})
	}
	return out, nil
}

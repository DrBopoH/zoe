package market

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
)

// История цен акций (этап F2). Источник — Yahoo chart без ключа (подтверждён зондом probe8, 17.11.2026): месячные закрытия за всю жизнь бумаги.
// Дневная история interval=1d&range=max Yahoo прореживает (у AAPL пришло 169 точек), поэтому берётся месяц — как для индексов бирж.
// Close — цена закрытия месяца с поправкой на сплиты, Adj — ещё и на дивиденды (по ней считается доходность).
// Грузится не всё сразу: при обновлении — топ StockTopN по капитализации, остальные по требованию (GET /api/market/stock).
const (
	SourceStockPrices = "stockprices"

	StockTopN   = 500            // сколько крупнейших компаний обновляет общий опрос
	StockMaxAge = 20 * time.Hour // история моложе этого не перекачивается: опрос раз в сутки не гоняет Yahoo впустую
)

// PriceBar — месяц одной бумаги: дата — первое число месяца.
type PriceBar struct {
	Ticker string  `json:"-"`
	Date   string  `json:"d"`
	Close  float64 `json:"c"`
	Adj    float64 `json:"a"`
}

// StockHistory — месячная история одного тикера (в записи Yahoo: класс акций через дефис, BRK-B).
func (c *Client) StockHistory(ctx context.Context, ticker string) ([]PriceBar, error) {
	if c.YahooBase == "" {
		return nil, fmt.Errorf("источник Yahoo отключён")
	}
	u := fmt.Sprintf("%s/v8/finance/chart/%s?interval=1mo&range=max", strings.TrimRight(c.YahooBase, "/"), url.PathEscape(ticker))
	body, err := c.getHeaders(ctx, u, map[string]string{"Accept": "application/json", "User-Agent": "Mozilla/5.0"})
	if err != nil {
		return nil, fmt.Errorf("Yahoo %s: %w", ticker, err)
	}
	bars, err := parseStockHistory(body, ticker)
	if err != nil {
		return nil, fmt.Errorf("Yahoo %s: %w", ticker, err)
	}
	return bars, nil
}

// parseStockHistory читает ответ графика Yahoo (interval=1mo). Месяц без закрытия пропускается; два отсчёта в одном месяце (последний,
// незавершённый, Yahoo иногда дублирует) — остаётся поздний. Нет поправленной цены — Adj равна Close.
func parseStockHistory(body []byte, ticker string) ([]PriceBar, error) {
	var r struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Close []*float64 `json:"close"`
					} `json:"quote"`
					AdjClose []struct {
						Adj []*float64 `json:"adjclose"`
					} `json:"adjclose"`
				} `json:"indicators"`
			} `json:"result"`
			Error *struct {
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("неверный ответ (начало: %q)", head160(body))
	}
	if r.Chart.Error != nil {
		return nil, fmt.Errorf("%s", r.Chart.Error.Description)
	}
	if len(r.Chart.Result) == 0 || len(r.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("в ответе нет графика (начало: %q)", head160(body))
	}
	res := r.Chart.Result[0]
	closes := res.Indicators.Quote[0].Close
	var adjs []*float64
	if len(res.Indicators.AdjClose) > 0 {
		adjs = res.Indicators.AdjClose[0].Adj
	}
	var out []PriceBar
	for i, t := range res.Timestamp {
		if i >= len(closes) || closes[i] == nil || *closes[i] <= 0 || math.IsNaN(*closes[i]) {
			continue
		}
		bar := PriceBar{Ticker: ticker, Date: time.Unix(t, 0).UTC().Add(12*time.Hour).Format("2006-01") + "-01", Close: *closes[i], Adj: *closes[i]}
		if i < len(adjs) && adjs[i] != nil && *adjs[i] > 0 && !math.IsNaN(*adjs[i]) {
			bar.Adj = *adjs[i]
		}
		if n := len(out); n > 0 && out[n-1].Date == bar.Date {
			out[n-1] = bar
			continue
		}
		out = append(out, bar)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("в ответе нет ни одного закрытия")
	}
	return out, nil
}

// StockPrices качает историю тикеров по одному и отдаёт каждую в save сразу: отмена или сбой на середине не теряют уже скачанное.
// Тикер без истории (Yahoo его не знает) пропускается; об ошибке источника сообщается, если без истории осталась пятая часть и больше.
func (c *Client) StockPrices(ctx context.Context, rep Reporter, tickers []string, save func([]PriceBar) error) Result {
	res := Result{Source: SourceStockPrices}
	if c.YahooBase == "" {
		return res
	}
	var errs []string
	for i, t := range tickers {
		if ctx.Err() != nil {
			return res
		}
		rep.report(i*100/len(tickers), t)
		bars, err := c.StockHistory(ctx, t)
		if err == nil {
			err = save(bars)
		}
		if err != nil {
			if ctx.Err() != nil {
				return res
			}
			res.Skipped++
			if len(errs) < 3 {
				errs = append(errs, err.Error())
			}
			continue
		}
		res.Fetched += len(bars)
	}
	if res.Skipped > 0 && res.Skipped*5 >= len(tickers) {
		res.Error = fmt.Sprintf("без истории %d из %d: %s", res.Skipped, len(tickers), strings.Join(errs, "; "))
	}
	return res
}

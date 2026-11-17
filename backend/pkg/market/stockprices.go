package market

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"
)

// История цен акций (этап F2). Источник — Yahoo chart без ключа (подтверждён зондом probe8, 17.11.2026): месячные закрытия за всю жизнь бумаги.
// Дневная история interval=1d&range=max Yahoo прореживает (у AAPL пришло 169 точек), поэтому берётся месяц — как для индексов бирж.
// Close — цена закрытия месяца с поправкой на сплиты, Adj — ещё и на дивиденды (по ней считается доходность).
// Грузится не всё сразу: при обновлении — топ StockTopN по капитализации, остальные по требованию (GET /api/market/stock).
const (
	SourceStockPrices = "stockprices"
	SourceStockFacts  = "stockfacts"

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

// Split — сплит акций: Ratio — во сколько раз стало больше акций (4:1 — 4, обратный 1:10 — 0,1). Date — день сплита.
type Split struct {
	Date  string  `json:"d"`
	Ratio float64 `json:"r"`
}

// StockHistory — месячная история одного тикера (в записи Yahoo: класс акций через дефис, BRK-B) и его сплиты.
// Цены Yahoo уже поправлены на сплиты, а число акций из отчётности — нет: сплиты нужны, чтобы привести акции к сегодняшней базе (см. CapSeries).
func (c *Client) StockHistory(ctx context.Context, ticker string) ([]PriceBar, []Split, error) {
	if c.YahooBase == "" {
		return nil, nil, fmt.Errorf("источник Yahoo отключён")
	}
	u := fmt.Sprintf("%s/v8/finance/chart/%s?interval=1mo&range=max&events=split", strings.TrimRight(c.YahooBase, "/"), url.PathEscape(ticker))
	body, err := c.getHeaders(ctx, u, map[string]string{"Accept": "application/json", "User-Agent": "Mozilla/5.0"})
	if err != nil {
		return nil, nil, fmt.Errorf("Yahoo %s: %w", ticker, err)
	}
	bars, splits, err := parseStockHistory(body, ticker)
	if err != nil {
		return nil, nil, fmt.Errorf("Yahoo %s: %w", ticker, err)
	}
	return bars, splits, nil
}

// parseStockHistory читает ответ графика Yahoo (interval=1mo). Месяц без закрытия пропускается; два отсчёта в одном месяце (последний,
// незавершённый, Yahoo иногда дублирует) — остаётся поздний. Нет поправленной цены — Adj равна Close.
func parseStockHistory(body []byte, ticker string) ([]PriceBar, []Split, error) {
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
				Events struct {
					Splits map[string]struct {
						Date        int64   `json:"date"`
						Numerator   float64 `json:"numerator"`
						Denominator float64 `json:"denominator"`
					} `json:"splits"`
				} `json:"events"`
			} `json:"result"`
			Error *struct {
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, nil, fmt.Errorf("неверный ответ (начало: %q)", head160(body))
	}
	if r.Chart.Error != nil {
		return nil, nil, fmt.Errorf("%s", r.Chart.Error.Description)
	}
	if len(r.Chart.Result) == 0 || len(r.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, nil, fmt.Errorf("в ответе нет графика (начало: %q)", head160(body))
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
		return nil, nil, fmt.Errorf("в ответе нет ни одного закрытия")
	}
	var splits []Split
	for _, sp := range res.Events.Splits {
		if sp.Date > 0 && sp.Numerator > 0 && sp.Denominator > 0 {
			splits = append(splits, Split{Date: time.Unix(sp.Date, 0).UTC().Format("2006-01-02"), Ratio: sp.Numerator / sp.Denominator})
		}
	}
	sort.Slice(splits, func(i, j int) bool { return splits[i].Date < splits[j].Date })
	return out, splits, nil
}

// StockPrices качает историю и сплиты тикеров по одному и отдаёт каждую в save сразу: отмена или сбой на середине не теряют уже скачанное.
// Тикер без истории (Yahoo его не знает) пропускается; об ошибке источника сообщается, если без истории осталась пятая часть и больше.
func (c *Client) StockPrices(ctx context.Context, rep Reporter, tickers []string, save func(ticker string, bars []PriceBar, splits []Split) error) Result {
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
		bars, splits, err := c.StockHistory(ctx, t)
		if err == nil {
			err = save(t, bars, splits)
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

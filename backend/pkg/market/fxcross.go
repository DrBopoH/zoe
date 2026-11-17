package market

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// НБУ перестал публиковать рубль после 31.03.2025 (проверено живым curl 15.11.2026), поэтому ряд rub_uah
// продолжается кросс-курсом: гривен за доллар (НБУ, последний известный на дату) ÷ рублей за доллар (Yahoo RUB=X).
const (
	yahooUSDRUB    = "RUB=X"
	crossMinGap    = 14 * 24 * time.Hour // ряд свежее — докручивать нечем и незачем
	crossYahooPath = "/v8/finance/chart/%s?interval=1d&range=5y"
)

// extendRUB дописывает к рублю НБУ точки кросс-курса за даты после его последней. Ошибка Yahoo не ломает ряд: он остаётся как есть.
func (c *Client) extendRUB(ctx context.Context, rub, usd []SeriesPoint, now time.Time) ([]SeriesPoint, error) {
	if c.YahooBase == "" || len(rub) == 0 || len(usd) == 0 {
		return rub, nil
	}
	last := rub[len(rub)-1].Date
	if t, err := time.Parse("2006-01-02", last); err != nil || now.Sub(t) < crossMinGap {
		return rub, nil
	}
	url := fmt.Sprintf("%s"+crossYahooPath, strings.TrimRight(c.YahooBase, "/"), yahooUSDRUB)
	body, err := c.getHeaders(ctx, url, map[string]string{"Accept": "application/json", "User-Agent": "Mozilla/5.0"})
	if err != nil {
		return rub, fmt.Errorf("Yahoo %s: %w", yahooUSDRUB, err)
	}
	quotes, err := parseYahooDaily(body)
	if err != nil {
		return rub, fmt.Errorf("Yahoo %s: %w", yahooUSDRUB, err)
	}
	out := append([]SeriesPoint(nil), rub...)
	for _, q := range quotes {
		if q.Date <= last {
			continue
		}
		i := sort.Search(len(usd), func(i int) bool { return usd[i].Date > q.Date }) - 1
		if i < 0 {
			continue
		}
		out = append(out, SeriesPoint{Series: SerRUBUAH, Date: q.Date, Value: usd[i].Value / q.Value})
	}
	return out, nil
}

// parseYahooDaily читает дневной график Yahoo: закрытие по датам (UTC), по возрастанию; пустые и нулевые закрытия пропускаются.
func parseYahooDaily(body []byte) ([]SeriesPoint, error) {
	var r struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Close []*float64 `json:"close"`
					} `json:"quote"`
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
	ts, closes := r.Chart.Result[0].Timestamp, r.Chart.Result[0].Indicators.Quote[0].Close
	byDate := map[string]float64{}
	for i, t := range ts {
		if i >= len(closes) || closes[i] == nil || *closes[i] <= 0 || math.IsNaN(*closes[i]) {
			continue
		}
		byDate[time.Unix(t, 0).UTC().Format("2006-01-02")] = *closes[i]
	}
	out := make([]SeriesPoint, 0, len(byDate))
	for d, v := range byDate {
		out = append(out, SeriesPoint{Date: d, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

package market

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// Пакеты Южной Кореи и Тайваня. Источники подтверждены зондом Дока 16.11.2026 (probe4).
// Корея: ставка Банка Кореи и инфляция — BIS (с 1999 и 1960-х), индекс KOSPI — Yahoo (^KS11, с 1997 года).
// Тайвань: у BIS нет ни ставки, ни инфляции Тайваня (в ответе одна Корея), поэтому только индекс TAIEX — Yahoo (^TWII, с 1997 года).
// Реальные эффективные курсы обеих валют — общий список reer.go, курс вона к гривне у НБУ не подтверждён и в набор не входит.
// Индекс — месячные закрытия, пересчитанные так, чтобы среднее 2015 года было 100 (как у остальных индексов набора, worldmarkets.go).
const (
	SourceKR = "kr"
	SourceTW = "tw"

	SerKRPolicy = "kr_policy" // ставка Банка Кореи (BIS), % — только даты смены значения
	SerKRCPI    = "kr_cpi"    // инфляция CPI г/г, % (BIS)

	yahooBaseYear = 2015
)

var (
	krSeries = []string{SerKRPolicy, SerKRCPI, SerWM("kr")}
	twSeries = []string{SerWM("tw")}
)

// Korea — ряды Кореи. Сбой части (BIS или Yahoo) и отдельного ряда не трогает остальное.
func (c *Client) Korea(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceKR}
	var out []SeriesPoint
	if c.BISBase != "" {
		pts, r := c.runJobs(ctx, rep, SourceKR, []packJob{bisPolicy(SerKRPolicy, "KR"), bisCPIJob(SerKRCPI, "KR")})
		out, res = append(out, pts...), mergeResult(res, r)
	}
	if c.YahooBase != "" {
		rep.report(90, "KOSPI")
		res = mergeResult(res, c.yahooJob(ctx, "^KS11", SerWM("kr"), &out))
	}
	res.Fetched = len(out)
	return out, res
}

// Taiwan — индекс TAIEX.
func (c *Client) Taiwan(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceTW}
	var out []SeriesPoint
	if c.YahooBase != "" {
		rep.report(30, "TAIEX")
		res = mergeResult(res, c.yahooJob(ctx, "^TWII", SerWM("tw"), &out))
	}
	res.Fetched = len(out)
	return out, res
}

// yahooJob добавляет ряд индекса в out; ошибка уходит в итог источника, как у остальных частей пакета.
func (c *Client) yahooJob(ctx context.Context, symbol, series string, out *[]SeriesPoint) Result {
	pts, err := c.yahooIndex(ctx, symbol, series)
	if err != nil {
		return Result{Error: err.Error()}
	}
	*out = append(*out, pts...)
	return Result{}
}

// yahooIndex — месячные закрытия индекса биржи с Yahoo, среднее 2015 года = 100.
func (c *Client) yahooIndex(ctx context.Context, symbol, series string) ([]SeriesPoint, error) {
	url := fmt.Sprintf("%s/v8/finance/chart/%s?interval=1mo&range=max", strings.TrimRight(c.YahooBase, "/"), strings.ReplaceAll(symbol, "^", "%5E"))
	// Yahoo отвечает отказом клиенту без браузерного User-Agent
	body, err := c.getHeaders(ctx, url, map[string]string{"Accept": "application/json", "User-Agent": "Mozilla/5.0"})
	if err != nil {
		return nil, fmt.Errorf("Yahoo %s: %w", symbol, err)
	}
	pts, err := parseYahooMonthly(body, series)
	if err != nil {
		return nil, fmt.Errorf("Yahoo %s: %w", symbol, err)
	}
	return pts, nil
}

// parseYahooMonthly читает ответ графика Yahoo (interval=1mo) и приводит индекс к среднему 2015 года = 100.
// Метка месяца у Yahoo — полночь по времени биржи (для Азии это конец прошлого дня по UTC), поэтому к ней прибавляется 12 часов, и месяц берётся из полученной даты.
func parseYahooMonthly(body []byte, series string) ([]SeriesPoint, error) {
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
	type obs struct {
		date string
		v    float64
	}
	var all []obs
	var baseSum float64
	var baseN int
	for i, t := range ts {
		if i >= len(closes) || closes[i] == nil || *closes[i] <= 0 || math.IsNaN(*closes[i]) {
			continue
		}
		d := time.Unix(t, 0).UTC().Add(12 * time.Hour)
		all = append(all, obs{d.Format("2006-01") + "-01", *closes[i]})
		if d.Year() == yahooBaseYear {
			baseSum += *closes[i]
			baseN++
		}
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("в ответе нет ни одного закрытия")
	}
	if baseN == 0 {
		return nil, fmt.Errorf("нет данных за %d год — не к чему приводить индекс", yahooBaseYear)
	}
	base := baseSum / float64(baseN)
	out := make([]SeriesPoint, len(all))
	for i, o := range all {
		out[i] = SeriesPoint{Series: series, Date: o.date, Value: math.Round(o.v/base*10000) / 100}
	}
	return out, nil
}

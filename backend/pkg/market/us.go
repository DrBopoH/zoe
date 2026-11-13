package market

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// США: ставка ФРС, инфляция и доходность 10-летних Treasury из FRED (fred.stlouisfed.org/graph/fredgraph.csv?id=…).
// Форма ответа проверена живым curl 12.11.2026: CSV «observation_date,ID», публичный, без ключа; пропуск в дневных рядах — «.».
const (
	SourceUS = "us"

	SerFedFunds  = "fed_funds"  // эффективная ставка ФРС (FEDFUNDS), % годовых, помесячно с 1954 года
	SerFedTarget = "fed_target" // верхняя граница целевого коридора ФРС (DFEDTARU), % — только даты смены значения, с 2008 года
	SerUSCPIYoY  = "us_cpi_yoy" // инфляция США к тому же месяцу прошлого года (считается из индекса CPIAUCSL), %
	SerUSCPIMoM  = "us_cpi_mom" // инфляция США к предыдущему месяцу, %
	SerUST10Y    = "ust_10y"    // доходность 10-летних Treasury (DGS10), % — последнее значение недели
)

// USFRED — все ряды США. Сбой одного ряда не убивает остальные: что скачалось — возвращается, ошибка в Result.Error.
func (c *Client) USFRED(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceUS}
	r := c.fredAll(ctx, rep, []fredJob{
		{"FEDFUNDS", "ставка ФРС"}, {"DFEDTARU", "целевая ставка"}, {"CPIAUCSL", "инфляция США"}, {"DGS10", "доходность Treasury"},
	})
	var out []SeriesPoint
	out = append(out, toPoints(SerFedFunds, r.Obs["FEDFUNDS"])...)
	out = append(out, changePoints(toPoints(SerFedTarget, r.Obs["DFEDTARU"]))...)
	yoy, mom := cpiChanges(r.Obs["CPIAUCSL"])
	out = append(out, yoy...)
	out = append(out, mom...)
	out = append(out, weekly(toPoints(SerUST10Y, r.Obs["DGS10"]))...)
	rep.report(95, "")
	res.Fetched, res.Skipped = len(out), r.Skipped
	res.Error = strings.Join(r.Errs, "; ")
	return out, res
}

type fredObs struct {
	Date  string
	Value float64
}

// fred — один ряд FRED. Строки с пропуском («.») пропускаются.
func (c *Client) fred(ctx context.Context, id string) ([]fredObs, int, error) {
	body, err := c.get(ctx, strings.TrimRight(c.FREDBase, "/")+"/graph/fredgraph.csv?id="+id)
	if err != nil {
		return nil, 0, fmt.Errorf("FRED %s: %w", id, err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(body))).ReadAll()
	if err != nil {
		return nil, 0, fmt.Errorf("FRED %s: %w", id, err)
	}
	if len(rows) < 2 || len(rows[0]) < 2 {
		return nil, 0, fmt.Errorf("FRED %s: пустой ответ", id)
	}
	skipped := 0
	var out []fredObs
	for _, r := range rows[1:] {
		if len(r) < 2 {
			skipped++
			continue
		}
		d := normDate(r[0])
		v, err := strconv.ParseFloat(strings.TrimSpace(r[1]), 64)
		if d == "" || err != nil || math.IsNaN(v) {
			skipped++
			continue
		}
		out = append(out, fredObs{d, v})
	}
	return out, skipped, nil
}

func toPoints(series string, obs []fredObs) []SeriesPoint {
	out := make([]SeriesPoint, 0, len(obs))
	for _, o := range obs {
		out = append(out, SeriesPoint{Series: series, Date: o.Date, Value: o.Value})
	}
	return out
}

func fredPoints(series string, obs []fredObs) []SeriesPoint { return toPoints(series, obs) }

// weekly оставляет последнее значение каждой недели (ISO): дневной ряд на 60 лет слишком тяжёл для графика.
func weekly(pts []SeriesPoint) []SeriesPoint {
	sort.Slice(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
	var out []SeriesPoint
	lastKey := ""
	for _, p := range pts {
		t, err := time.Parse("2006-01-02", p.Date)
		if err != nil {
			continue
		}
		y, w := t.ISOWeek()
		key := fmt.Sprintf("%d-%02d", y, w)
		if key == lastKey {
			out[len(out)-1] = p
		} else {
			out = append(out, p)
			lastKey = key
		}
	}
	return out
}

// cpiChanges считает инфляцию США из индекса: к тому же месяцу прошлого года и к предыдущему месяцу, %.
func cpiChanges(obs []fredObs) (yoy, mom []SeriesPoint) {
	idx := map[string]float64{}
	for _, o := range obs {
		idx[o.Date[:7]] = o.Value
	}
	sort.Slice(obs, func(i, j int) bool { return obs[i].Date < obs[j].Date })
	for _, o := range obs {
		t, err := time.Parse("2006-01-02", o.Date)
		if err != nil {
			continue
		}
		d := t.Format("2006-01") + "-01"
		if prev, ok := idx[t.AddDate(-1, 0, 0).Format("2006-01")]; ok && prev > 0 {
			yoy = append(yoy, SeriesPoint{Series: SerUSCPIYoY, Date: d, Value: (o.Value/prev - 1) * 100})
		}
		if prev, ok := idx[time.Date(t.Year(), t.Month()-1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01")]; ok && prev > 0 {
			mom = append(mom, SeriesPoint{Series: SerUSCPIMoM, Date: d, Value: (o.Value/prev - 1) * 100})
		}
	}
	return yoy, mom
}

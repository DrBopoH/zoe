package market

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Мировые рынки акций и оценка рынка США. Источники подтверждены зондами 16.11.2026 (probe7, probe8):
// индексы акций ОЭСР из FRED — помесячно, 2015 = 100, до 2026-09 (у еврозоны ряд обрывается в 2026-01, поэтому её нет);
// индекс Баффета — рыночная стоимость акций (NCBEILQ027S, млн $, поквартально) к ВВП (GDP, млрд $); CAPE и дивидендная доходность — таблица Шиллера.
// Шиллер перестал обновлять данные: PE10 и дивиденды после 2023 года нули, они пропускаются (на экране — история до 2023).
// Не взяли: Yahoo (на все запросы отвечает «Too Many Requests»), MOVE (источника нет), Stoxx 600, отдельные индексы бирж.
const (
	SourceWorld = "world"

	SerWMPrefix   = "wm_"         // wm_<cc> — индекс акций страны, 2015 = 100
	SerValBuffett = "val_buffett" // рыночная стоимость акций США к ВВП, %
	SerValCAPE    = "val_cape"    // CAPE (PE10) S&P 500, раз — до 2023-09
	SerValDivYld  = "val_divy"    // дивидендная доходность S&P 500, % — до 2023-06
)

// worldIndexCC — страны индексов ОЭСР: код FRED SPASTT01<CC>M661N
var worldIndexCC = []string{"us", "jp", "gb", "de", "fr", "ch", "ca", "se", "no", "pl", "cn", "tr"}

// SerWM — id ряда индекса акций страны
func SerWM(cc string) string { return SerWMPrefix + cc }

var worldSeries = func() []string {
	out := make([]string, 0, len(worldIndexCC)+3)
	for _, cc := range worldIndexCC {
		out = append(out, SerWM(cc))
	}
	return append(out, SerValBuffett, SerValCAPE, SerValDivYld)
}()

// World — индексы акций стран и показатели оценки рынка США. Сбой части (ряд FRED, Шиллер) не трогает остальное, ошибка — в Result.Error.
func (c *Client) World(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceWorld}
	var errs []string
	var out []SeriesPoint
	if c.FREDBase != "" {
		jobs := make([]fredJob, 0, len(worldIndexCC)+2)
		for _, cc := range worldIndexCC {
			jobs = append(jobs, fredJob{ID: "SPASTT01" + strings.ToUpper(cc) + "M661N", Note: "акции: " + strings.ToUpper(cc), Series: SerWM(cc)})
		}
		jobs = append(jobs, fredJob{ID: "NCBEILQ027S", Note: "стоимость акций США"}, fredJob{ID: "GDP", Note: "ВВП США"})
		r := c.fredAll(ctx, rep, jobs)
		for _, cc := range worldIndexCC {
			out = append(out, toPoints(SerWM(cc), r.Obs["SPASTT01"+strings.ToUpper(cc)+"M661N"])...)
		}
		out = append(out, buffett(r.Obs["NCBEILQ027S"], r.Obs["GDP"])...)
		res.Skipped, res.Tail = r.Skipped, r.Tail
		errs = append(errs, r.Errs...)
	}
	rep.report(92, "Шиллер: CAPE и дивиденды")
	if c.ShillerBase != "" {
		pts, err := c.shillerValuation(ctx)
		if err != nil {
			errs = append(errs, err.Error())
		}
		out = append(out, pts...)
	}
	res.Fetched, res.Error = len(out), strings.Join(errs, "; ")
	return out, res
}

// buffett — рыночная стоимость акций (млн $) к ВВП (млрд $) на одну и ту же квартальную дату, %. Нет пары на дату — точки нет.
func buffett(mcap, gdp []fredObs) []SeriesPoint {
	g := make(map[string]float64, len(gdp))
	for _, o := range gdp {
		g[o.Date] = o.Value
	}
	var out []SeriesPoint
	for _, o := range mcap {
		if d := g[o.Date]; d > 0 && o.Value > 0 {
			out = append(out, SeriesPoint{Series: SerValBuffett, Date: o.Date, Value: math.Round(o.Value/1000/d*10000) / 100})
		}
	}
	return out
}

// shillerValuation — CAPE (колонка PE10) и дивидендная доходность (Dividend к SP500, в процентах). Нулевые значения Шиллера — «нет данных».
func (c *Client) shillerValuation(ctx context.Context) ([]SeriesPoint, error) {
	rows, err := c.shillerRows(ctx)
	if err != nil {
		return nil, err
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.TrimSpace(h)] = i
	}
	ip, id, ipe := col["SP500"], col["Dividend"], col["PE10"]
	if ip == 0 || id == 0 || ipe == 0 {
		return nil, fmt.Errorf("Шиллер: нет колонок SP500, Dividend, PE10")
	}
	num := func(r []string, i int) float64 {
		if i >= len(r) {
			return 0
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(r[i]), 64)
		if err != nil || math.IsNaN(v) || v < 0 {
			return 0
		}
		return v
	}
	var out []SeriesPoint
	for _, r := range rows[1:] {
		if len(r) == 0 {
			continue
		}
		d := normDate(r[0])
		if d == "" {
			continue
		}
		if pe := num(r, ipe); pe > 0 {
			out = append(out, SeriesPoint{Series: SerValCAPE, Date: d, Value: math.Round(pe*100) / 100})
		}
		if px, dv := num(r, ip), num(r, id); px > 0 && dv > 0 {
			out = append(out, SeriesPoint{Series: SerValDivYld, Date: d, Value: math.Round(dv/px*10000) / 100})
		}
	}
	return out, nil
}

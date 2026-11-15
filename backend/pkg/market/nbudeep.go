package market

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Глубокий слой НБУ: резервы и денежные агрегаты. Адреса и поля сняты живым зондом Дока 15.11.2026 (bank.gov.ua/NBUStatService/v1/statdirectory):
//
//	res       — без даты вся история помесячно с 2017-10 (358 КБ); строки dt, id_api, tzep, value; официальные резервы — id_api RES_OffReserveAssets, млн долларов;
//	monetary  — без даты вся история помесячно с 2016-06 (5 МБ); строки dt, id_api, txt, k076, value, млн гривен; агрегат M3 — id_api «M3», M0 — по названию «(M0)».
//
// Не взяли: депозиты и кредиты (набор отдаёт объёмы с 43 тысячами строк за месяц, а не ставки), платёжный баланс (7,5 МБ за год), ликвидность (только по дням),
// «dollar_info» (отдаёт тот же официальный курс). Ставки банков, платёжный баланс и компоненты инфляции — после отдельной разведки.
const (
	SerUAReserves = "ua_reserves" // международные резервы НБУ, млрд долларов, помесячно
	SerUAM0       = "ua_m0"       // наличные вне банков (M0), млрд гривен, помесячно
	SerUAM3       = "ua_m3"       // денежная масса M3, млрд гривен, помесячно
	SerUAM3YoY    = "ua_m3_yoy"   // рост M3 к тому же месяцу прошлого года, %

	nbuReservesID = "RES_OFFRESERVEASSETS"
)

var nbuDeepSeries = []string{SerUAReserves, SerUAM0, SerUAM3, SerUAM3YoY}

// MacroAll — ставки и инфляция НБУ плюс глубокий слой (резервы, M0, M3). Сбой любой части не трогает остальные: ошибки склеиваются в Result.
func (c *Client) MacroAll(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	out, res := c.MacroNBU(ctx, rep)
	deep, dres := c.nbuDeep(ctx, rep)
	out = append(out, deep...)
	res = mergeResult(res, dres)
	res.Fetched = len(out)
	return out, res
}

func (c *Client) nbuDeep(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceMacro}
	var out []SeriesPoint
	var errs []string

	rep.report(96, "резервы")
	if pts, sk, err := c.nbuReserves(ctx); err != nil {
		errs = append(errs, err.Error())
	} else {
		out = append(out, pts...)
		res.Skipped += sk
	}
	rep.report(98, "денежные агрегаты")
	if pts, sk, err := c.nbuMonetary(ctx); err != nil {
		errs = append(errs, err.Error())
	} else {
		out = append(out, pts...)
		res.Skipped += sk
	}
	res.Fetched, res.Error = len(out), strings.Join(errs, "; ")
	return out, res
}

// nbuReserves — официальные резервные активы: млн долларов → млрд. Если у ряда есть строки в долларах, берутся только они.
func (c *Client) nbuReserves(ctx context.Context) ([]SeriesPoint, int, error) {
	body, err := c.get(ctx, c.macroURL("res", "json"))
	if err != nil {
		return nil, 0, fmt.Errorf("резервы НБУ: %w", err)
	}
	recs, err := records(body)
	if err != nil {
		return nil, 0, fmt.Errorf("резервы НБУ: %w", err)
	}
	var usd, any []SeriesPoint
	skipped := 0
	for _, r := range recs {
		d := r.date("dt")
		if !strings.EqualFold(r.str("id_api"), nbuReservesID) || d == "" || r.str("value") == "" {
			skipped++
			continue
		}
		p := SeriesPoint{Series: SerUAReserves, Date: d[:8] + "01", Value: round2(r.num("value") / 1000)}
		any = append(any, p)
		if strings.Contains(strings.ToUpper(r.str("tzep")), "USD") {
			usd = append(usd, p)
		}
	}
	if len(usd) > 0 {
		any = usd
	}
	if len(any) == 0 {
		return nil, skipped, fmt.Errorf("резервы НБУ: в ответе нет ряда официальных резервов")
	}
	return sortedUnique(any), skipped, nil
}

// nbuMonetary — M3 и M0 (млн гривен → млрд) и рост M3 за год. Строка итога (k076 = «Total») предпочтительнее разбивок по валюте.
func (c *Client) nbuMonetary(ctx context.Context) ([]SeriesPoint, int, error) {
	body, err := c.get(ctx, c.macroURL("monetary", "json"))
	if err != nil {
		return nil, 0, fmt.Errorf("денежные агрегаты НБУ: %w", err)
	}
	recs, err := records(body)
	if err != nil {
		return nil, 0, fmt.Errorf("денежные агрегаты НБУ: %w", err)
	}
	type key struct{ series, date string }
	total := map[key]float64{}
	other := map[key]float64{}
	skipped := 0
	for _, r := range recs {
		id := ""
		switch {
		case strings.EqualFold(r.str("id_api"), "M3"):
			id = SerUAM3
		case strings.EqualFold(r.str("id_api"), "M0") || strings.Contains(r.str("txt"), "(M0)"):
			id = SerUAM0
		}
		d := r.date("dt")
		if id == "" || d == "" || r.str("value") == "" {
			skipped++
			continue
		}
		k := key{id, d[:8] + "01"}
		if strings.EqualFold(r.str("k076"), "total") {
			total[k] = r.num("value")
		} else if _, ok := other[k]; !ok {
			other[k] = r.num("value")
		}
	}
	level := map[string][]SeriesPoint{}
	for k, v := range other {
		if _, ok := total[k]; !ok {
			total[k] = v
		}
	}
	for k, v := range total {
		level[k.series] = append(level[k.series], SeriesPoint{Series: k.series, Date: k.date, Value: round2(v / 1000)})
	}
	var errs []string
	for _, id := range []string{SerUAM3, SerUAM0} {
		if len(level[id]) == 0 {
			errs = append(errs, id)
		}
	}
	var out []SeriesPoint
	for _, id := range []string{SerUAM0, SerUAM3} {
		out = append(out, sortedUnique(level[id])...)
	}
	if m3 := sortedUnique(level[SerUAM3]); len(m3) > 0 {
		obs := make([]fredObs, len(m3))
		for i, p := range m3 {
			obs[i] = fredObs{Date: p.Date, Value: p.Value}
		}
		yoy, _ := indexChanges(obs, SerUAM3YoY, "")
		for i := range yoy {
			yoy[i].Value = round2(yoy[i].Value)
		}
		out = append(out, yoy...)
	}
	if len(errs) > 0 {
		return out, skipped, fmt.Errorf("денежные агрегаты НБУ: в ответе нет рядов %s", strings.Join(errs, ", "))
	}
	return out, skipped, nil
}

func sortedUnique(pts []SeriesPoint) []SeriesPoint {
	sort.Slice(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
	var out []SeriesPoint
	for _, p := range pts {
		if n := len(out); n > 0 && out[n-1].Date == p.Date {
			out[n-1] = p
			continue
		}
		out = append(out, p)
	}
	return out
}

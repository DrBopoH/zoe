package market

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Макроэкономика НБУ: ставки (bank.gov.ua/NBUStatService/v1/statdirectory/key) и индекс потребительских цен
// (…/statdirectory/inflation). Форма ответов проверена живым curl: ряды ставок приходят одним запросом без параметров
// (ежедневно с 1992 года), инфляция — только окнами start/end (целиком сервер отказывает: «надто великий dataset»).
const (
	SourceMacro = "macro"

	SerPolicyRate     = "policy_rate"     // облікова ставка НБУ, %
	SerOvernightCD    = "overnight_cd"    // ставка по депозитным сертификатам овернайт, %
	SerOvernightLoans = "overnight_loans" // ставка по кредитам овернайт, %
	SerCPIYoY         = "cpi_yoy"         // инфляция (ИПЦ) к тому же месяцу прошлого года, %
	SerCPIMoM         = "cpi_mom"         // инфляция к предыдущему месяцу, %
	SerCoreYoY        = "core_yoy"        // базовая инфляция к тому же месяцу прошлого года, %
	SerCoreMoM        = "core_mom"        // базовая инфляция к предыдущему месяцу, %

	// инфляция идёт окнами по 10 лет: данные начинаются с 2007 года, окно побольше сервер не отдаёт
	cpiFirstYear = 2000
	cpiWindow    = 10
)

// SeriesPoint — одна точка макро-ряда. Ряды ставок хранятся только в датах смены значения (значение действует
// до следующей точки), инфляция — помесячно (дата — первое число месяца).
type SeriesPoint struct {
	Series string  `json:"series"`
	Date   string  `json:"d"` // YYYY-MM-DD
	Value  float64 `json:"v"`
}

var keyRateSeries = map[string]string{
	"KEY_POLICYRATE":         SerPolicyRate,
	"KEY_RATEOVERNIGHTCD":    SerOvernightCD,
	"KEY_RATEOVERNIGHTLOANS": SerOvernightLoans,
}

// MacroNBU — ставки и инфляция НБУ. Сбой одной части не убивает другую: что скачалось — возвращается, ошибка в Result.Error.
func (c *Client) MacroNBU(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceMacro}
	var out []SeriesPoint
	var errs []string

	rep.report(5, "ключевые ставки")
	rates, skipped, err := c.keyRates(ctx)
	res.Skipped += skipped
	if err != nil {
		errs = append(errs, err.Error())
	}
	out = append(out, rates...)

	now := c.now()
	step := 0
	windows := 0
	for y := cpiFirstYear; y <= now.Year(); y += cpiWindow {
		windows++
	}
	for y := cpiFirstYear; y <= now.Year(); y += cpiWindow {
		rep.report(30+step*60/windows, fmt.Sprintf("инфляция %d–%d", y, min(y+cpiWindow-1, now.Year())))
		step++
		end := time.Date(min(y+cpiWindow-1, now.Year()), 12, 31, 0, 0, 0, 0, time.UTC)
		if end.After(now) {
			end = now
		}
		pts, sk, err := c.inflation(ctx, time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), end)
		res.Skipped += sk
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		out = append(out, pts...)
	}
	if !hasSeries(out, SerCPIYoY) && len(errs) == 0 {
		errs = append(errs, "инфляция: в ответе нет общего индекса потребительских цен")
	}
	rep.report(95, "")
	res.Fetched = len(out)
	res.Error = strings.Join(errs, "; ")
	return out, res
}

func hasSeries(pts []SeriesPoint, id string) bool {
	for _, p := range pts {
		if p.Series == id {
			return true
		}
	}
	return false
}

func (c *Client) macroURL(path, query string) string {
	return strings.TrimRight(c.NBUBase, "/") + "/NBUStatService/v1/statdirectory/" + path + "?" + query
}

// keyRates — ряды ключевых ставок, сжатые до дат смены значения.
func (c *Client) keyRates(ctx context.Context) ([]SeriesPoint, int, error) {
	body, err := c.get(ctx, c.macroURL("key", "json"))
	if err != nil {
		return nil, 0, fmt.Errorf("ставки НБУ: %w", err)
	}
	recs, err := records(body)
	if err != nil {
		return nil, 0, fmt.Errorf("ставки НБУ: %w", err)
	}
	skipped := 0
	daily := map[string][]SeriesPoint{}
	for _, r := range recs {
		id, ok := keyRateSeries[strings.ToUpper(r.str("id_api"))]
		d := r.date("dt")
		if !ok || d == "" || r.str("value") == "" {
			skipped++
			continue
		}
		daily[id] = append(daily[id], SeriesPoint{Series: id, Date: d, Value: r.num("value")})
	}
	var out []SeriesPoint
	for _, id := range []string{SerPolicyRate, SerOvernightCD, SerOvernightLoans} {
		out = append(out, changePoints(daily[id])...)
	}
	if len(out) == 0 {
		return nil, skipped, fmt.Errorf("ставки НБУ: в ответе нет ни одного ряда")
	}
	return out, skipped, nil
}

// changePoints оставляет первую точку и каждую смену значения (вход — один ряд, порядок любой).
func changePoints(pts []SeriesPoint) []SeriesPoint {
	sort.Slice(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
	var out []SeriesPoint
	for i, p := range pts {
		if i == 0 || p.Value != out[len(out)-1].Value {
			out = append(out, p)
		}
	}
	return out
}

// inflation — общий и базовый ИПЦ за окно дат. Из десятков разбивок (регионы, категории, цены производителей)
// берутся только строки итога: ku пуст, mcrd081 = «Total», mcrk110 пуст, название «Індекс споживчих цін» или «Базовий …».
func (c *Client) inflation(ctx context.Context, from, to time.Time) ([]SeriesPoint, int, error) {
	q := fmt.Sprintf("period=m&start=%s&end=%s&json", from.Format("20060102"), to.Format("20060102"))
	body, err := c.get(ctx, c.macroURL("inflation", q))
	if err != nil {
		return nil, 0, fmt.Errorf("инфляция %d–%d: %w", from.Year(), to.Year(), err)
	}
	recs, err := records(body)
	if err != nil {
		return nil, 0, fmt.Errorf("инфляция %d–%d: %w", from.Year(), to.Year(), err)
	}
	skipped := 0
	var out []SeriesPoint
	for _, r := range recs {
		if r.str("ku") != "" || r.str("mcrk110") != "" || !strings.EqualFold(r.str("mcrd081"), "total") {
			skipped++
			continue
		}
		name := strings.ToLower(r.str("txt", "txten"))
		core := strings.HasPrefix(name, "базов") || strings.HasPrefix(name, "core")
		if !core && !strings.HasPrefix(name, "індекс споживчих") && !strings.HasPrefix(name, "consumer price") {
			skipped++
			continue
		}
		id := ""
		switch strings.ToUpper(r.str("tzep")) {
		case "DTPY_":
			id = SerCPIYoY
			if core {
				id = SerCoreYoY
			}
		case "PCPM_":
			id = SerCPIMoM
			if core {
				id = SerCoreMoM
			}
		default:
			skipped++
			continue
		}
		d := r.date("dt")
		if d == "" || r.str("value") == "" {
			skipped++
			continue
		}
		out = append(out, SeriesPoint{Series: id, Date: d[:8] + "01", Value: r.num("value")})
	}
	return out, skipped, nil
}

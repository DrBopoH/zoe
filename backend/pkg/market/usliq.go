package market

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Ликвидность США из FRED: деньги в экономике (M2), баланс ФРС, обратное РЕПО, счёт Казначейства и ставка SOFR.
// Из них считается «чистая ликвидность» — сколько денег ФРС держит в системе за вычетом того, что застряло на счёте Казначейства и в РЕПО.
const (
	SourceUSLiq = "usliq"

	SerUSM2     = "us_m2"     // денежная масса M2, млрд $ (M2SL)
	SerUSM2YoY  = "us_m2_yoy" // M2 к прошлому году, %
	SerUSFedBS  = "us_fedbs"  // баланс ФРС, млрд $ (WALCL, раз в неделю; в FRED — млн)
	SerUSRRP    = "us_rrp"    // обратное РЕПО ФРС, млрд $ (RRPONTSYD) — последнее значение недели
	SerUSTGA    = "us_tga"    // счёт Казначейства в ФРС, млрд $ (WTREGEN, раз в неделю; в FRED — млн)
	SerUSNetLiq = "us_netliq" // чистая ликвидность = баланс ФРС − счёт Казначейства − обратное РЕПО, млрд $
	SerUSSOFR   = "us_sofr"   // ставка SOFR (обеспеченная овернайт), % — последнее значение недели
)

// scalePoints делит значения ряда на k и меняет его id: FRED отдаёт баланс ФРС и счёт Казначейства в миллионах, а РЕПО в миллиардах.
func scalePoints(pts []SeriesPoint, id string, k float64) []SeriesPoint {
	out := make([]SeriesPoint, len(pts))
	for i, p := range pts {
		out[i] = SeriesPoint{Series: id, Date: p.Date, Value: p.Value / k}
	}
	return out
}

// netLiquidity — баланс − счёт Казначейства − обратное РЕПО на даты баланса (недельные, по средам). Счёт и РЕПО берутся последние не позже
// этой даты, но не старше 10 дней: чтобы дыра в одном ряду не давала «ликвидность» из устаревших кусков. Все три ряда — в млрд $.
func netLiquidity(fedbs, tga, rrp []SeriesPoint) []SeriesPoint {
	latest := func(pts []SeriesPoint, date string) (float64, bool) {
		i := sort.Search(len(pts), func(i int) bool { return pts[i].Date > date }) - 1
		if i < 0 || daysBetween(pts[i].Date, date) > 10 {
			return 0, false
		}
		return pts[i].Value, true
	}
	sort.Slice(fedbs, func(i, j int) bool { return fedbs[i].Date < fedbs[j].Date })
	sort.Slice(tga, func(i, j int) bool { return tga[i].Date < tga[j].Date })
	sort.Slice(rrp, func(i, j int) bool { return rrp[i].Date < rrp[j].Date })
	var out []SeriesPoint
	for _, b := range fedbs {
		t, ok1 := latest(tga, b.Date)
		r, ok2 := latest(rrp, b.Date)
		if ok1 && ok2 {
			out = append(out, SeriesPoint{Series: SerUSNetLiq, Date: b.Date, Value: b.Value - t - r})
		}
	}
	return out
}

// USLiquidity — ряды ликвидности США. Сбой одного ряда не убивает остальные (чистая ликвидность тогда просто не считается), ошибка — в Result.Error.
func (c *Client) USLiquidity(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceUSLiq}
	r := c.fredAll(ctx, rep, []fredJob{
		{"M2SL", "денежная масса M2", ""}, {"WALCL", "баланс ФРС", ""}, {"RRPONTSYD", "обратное РЕПО", ""}, {"WTREGEN", "счёт Казначейства", ""}, {"SOFR", "ставка SOFR", SerUSSOFR},
	})
	var out []SeriesPoint
	out = append(out, toPoints(SerUSM2, r.Obs["M2SL"])...)
	m2yoy, _ := indexChanges(r.Obs["M2SL"], SerUSM2YoY, "")
	out = append(out, m2yoy...)
	fedbs := scalePoints(toPoints("", r.Obs["WALCL"]), SerUSFedBS, 1000)
	tga := scalePoints(toPoints("", r.Obs["WTREGEN"]), SerUSTGA, 1000)
	rrp := weekly(toPoints(SerUSRRP, r.Obs["RRPONTSYD"]))
	out = append(out, fedbs...)
	out = append(out, tga...)
	out = append(out, rrp...)
	out = append(out, netLiquidity(fedbs, tga, rrp)...)
	out = append(out, weekly(toPoints(SerUSSOFR, r.Obs["SOFR"]))...)
	rep.report(95, "")
	res.Fetched, res.Skipped = len(out), r.Skipped
	res.Error, res.Tail = strings.Join(r.Errs, "; "), r.Tail
	return out, res
}

// daysBetween — сколько дней от from до to (to не раньше from); непонятная дата — «очень много», чтобы точка не считалась свежей.
func daysBetween(from, to string) int {
	a, err1 := time.Parse("2006-01-02", from)
	b, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil {
		return 1 << 30
	}
	return int(b.Sub(a).Hours() / 24)
}

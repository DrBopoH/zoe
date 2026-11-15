package market

import (
	"context"
	"strings"
)

// Рынок труда США из FRED: занятость, заявки на пособие, вакансии, зарплаты. Безработица (UNRATE) уже лежит в источнике «usecon».
const (
	SourceUSJobs = "usjobs"

	SerUSNFP    = "us_nfp"       // прирост занятых вне сельского хозяйства за месяц, тыс. человек (разность PAYEMS)
	SerUSClaims = "us_claims"    // первичные заявки на пособие по безработице за неделю, человек (ICSA)
	SerUSJOLTS  = "us_jolts"     // открытые вакансии, тыс. (JTSJOL)
	SerUSWages  = "us_wages_yoy" // средний часовой заработок к прошлому году, % (CES0500000003)
	SerUSU6     = "us_u6"        // широкая безработица U-6 (с неполной занятостью), % (U6RATE)
	SerUSECIYoY = "us_eci_yoy"   // стоимость рабочей силы (ECI, раз в квартал) к прошлому году, % (ECIALLCIV)
)

// USJobs — ряды рынка труда США. Сбой одного ряда не убивает остальные, ошибка — в Result.Error.
func (c *Client) USJobs(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceUSJobs}
	r := c.fredAll(ctx, rep, []fredJob{
		{"PAYEMS", "занятость", ""}, {"ICSA", "заявки на пособие", SerUSClaims}, {"JTSJOL", "вакансии", SerUSJOLTS},
		{"CES0500000003", "зарплаты", ""}, {"U6RATE", "безработица U-6", SerUSU6}, {"ECIALLCIV", "стоимость рабочей силы", ""},
	})
	var out []SeriesPoint
	out = append(out, levelChanges(r.Obs["PAYEMS"], SerUSNFP)...)
	out = append(out, toPoints(SerUSClaims, r.Obs["ICSA"])...)
	out = append(out, toPoints(SerUSJOLTS, r.Obs["JTSJOL"])...)
	wy, _ := indexChanges(r.Obs["CES0500000003"], SerUSWages, "")
	out = append(out, wy...)
	out = append(out, toPoints(SerUSU6, r.Obs["U6RATE"])...)
	ey, _ := indexChanges(r.Obs["ECIALLCIV"], SerUSECIYoY, "")
	out = append(out, ey...)
	rep.report(95, "")
	res.Fetched, res.Skipped = len(out), r.Skipped
	res.Error, res.Tail = strings.Join(r.Errs, "; "), r.Tail
	return out, res
}

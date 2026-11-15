package market

import (
	"context"
	"strings"
)

// Активность США из FRED: потребитель, производство, стройка и цены на дома.
const (
	SourceUSAct = "usact"

	SerUSRetailYoY    = "us_retail_yoy"    // розничные продажи к прошлому году, % (RSAFS)
	SerUSIndproYoY    = "us_indpro_yoy"    // промышленное производство к прошлому году, % (INDPRO)
	SerUSHousing      = "us_housing"       // начатое строительство жилья, тыс. домов в год (HOUST)
	SerUSHomePriceYoY = "us_homeprice_yoy" // цены на дома Кейса — Шиллера к прошлому году, % (CSUSHPINSA)
)

// USActivity — ряды активности США. Сбой одного ряда не убивает остальные, ошибка — в Result.Error.
func (c *Client) USActivity(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceUSAct}
	r := c.fredAll(ctx, rep, []fredJob{
		{"RSAFS", "розничные продажи", ""}, {"INDPRO", "промышленность", ""}, {"HOUST", "стройка жилья", SerUSHousing}, {"CSUSHPINSA", "цены на дома", ""},
	})
	var out []SeriesPoint
	for _, j := range []struct{ id, series string }{{"RSAFS", SerUSRetailYoY}, {"INDPRO", SerUSIndproYoY}, {"CSUSHPINSA", SerUSHomePriceYoY}} {
		y, _ := indexChanges(r.Obs[j.id], j.series, "")
		out = append(out, y...)
	}
	out = append(out, toPoints(SerUSHousing, r.Obs["HOUST"])...)
	rep.report(95, "")
	res.Fetched, res.Skipped = len(out), r.Skipped
	res.Error, res.Tail = strings.Join(r.Errs, "; "), r.Tail
	return out, res
}

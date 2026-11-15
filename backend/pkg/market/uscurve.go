package market

import (
	"context"
	"strings"
)

// Кривая доходности Treasury из FRED (тот же публичный CSV без ключа, что и у остальных рядов США): все сроки, кроме 10 лет —
// 10-летние уже лежат в источнике «us» (ust_10y). Дневные ряды сводятся к последнему значению недели.
const (
	SourceUSCurve = "uscurve"

	SerUST1M      = "ust_1m"      // доходность 1-месячных Treasury (DGS1MO), %
	SerUST3M      = "ust_3m"      // 3-месячных (DGS3MO), %
	SerUST6M      = "ust_6m"      // 6-месячных (DGS6MO), %
	SerUST1Y      = "ust_1y"      // 1-летних (DGS1), %
	SerUST2Y      = "ust_2y"      // 2-летних (DGS2), %
	SerUST3Y      = "ust_3y"      // 3-летних (DGS3), %
	SerUST5Y      = "ust_5y"      // 5-летних (DGS5), %
	SerUST7Y      = "ust_7y"      // 7-летних (DGS7), %
	SerUST20Y     = "ust_20y"     // 20-летних (DGS20), %
	SerUST30Y     = "ust_30y"     // 30-летних (DGS30), %
	SerUSSpread3M = "us_spread3m" // 10-летние минус 3-месячные Treasury (T10Y3M), п. п. — классический предвестник рецессии
)

// USCurve — ряды кривой доходности. Как у USFRED: сбой одного ряда не убивает остальные, ошибка — в Result.Error.
func (c *Client) USCurve(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceUSCurve}
	jobs := []fredJob{
		{"DGS1MO", "1 месяц", SerUST1M}, {"DGS3MO", "3 месяца", SerUST3M}, {"DGS6MO", "6 месяцев", SerUST6M}, {"DGS1", "1 год", SerUST1Y},
		{"DGS2", "2 года", SerUST2Y}, {"DGS3", "3 года", SerUST3Y}, {"DGS5", "5 лет", SerUST5Y}, {"DGS7", "7 лет", SerUST7Y},
		{"DGS20", "20 лет", SerUST20Y}, {"DGS30", "30 лет", SerUST30Y}, {"T10Y3M", "10 лет минус 3 месяца", SerUSSpread3M},
	}
	r := c.fredAll(ctx, rep, jobs)
	var out []SeriesPoint
	for _, j := range jobs {
		out = append(out, weekly(toPoints(j.Series, r.Obs[j.ID]))...)
	}
	rep.report(95, "")
	res.Fetched, res.Skipped = len(out), r.Skipped
	res.Error, res.Tail = strings.Join(r.Errs, "; "), r.Tail
	return out, res
}

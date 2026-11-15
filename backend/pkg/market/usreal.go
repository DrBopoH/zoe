package market

import (
	"context"
	"strings"
)

// Реальные ставки и ожидаемая инфляция США из FRED: что рынок платит сверх инфляции и какую инфляцию закладывает в облигации.
const (
	SourceUSReal = "usreal"

	SerUSReal5    = "us_real5"    // реальная доходность 5-летних TIPS (DFII5), %
	SerUSReal10   = "us_real10"   // реальная доходность 10-летних TIPS (DFII10), %
	SerUSBE5      = "us_be5"      // ожидаемая инфляция на 5 лет — брейкивен (T5YIE), %
	SerUSBE10     = "us_be10"     // ожидаемая инфляция на 10 лет — брейкивен (T10YIE), %
	SerUSFwd5y5y  = "us_fwd5y5y"  // ожидаемая инфляция через 5 лет на следующие 5 (T5YIFR), %
	SerUSTermPrem = "us_termprem" // срочная премия 10-летних Treasury по модели ФРБ Нью-Йорка (THREEFYTP10), п. п.
	SerUSMortgage = "us_mortgage" // ставка 30-летней ипотеки (MORTGAGE30US, Freddie Mac), % — раз в неделю
)

// USReal — реальные ставки, брейкивены и ипотека. Сбой одного ряда не убивает остальные, ошибка — в Result.Error.
func (c *Client) USReal(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceUSReal}
	jobs := []fredJob{
		{"DFII5", "реальная 5 лет", SerUSReal5}, {"DFII10", "реальная 10 лет", SerUSReal10}, {"T5YIE", "брейкивен 5 лет", SerUSBE5},
		{"T10YIE", "брейкивен 10 лет", SerUSBE10}, {"T5YIFR", "инфляция 5y5y", SerUSFwd5y5y}, {"THREEFYTP10", "срочная премия", SerUSTermPrem},
		{"MORTGAGE30US", "ипотека 30 лет", SerUSMortgage},
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

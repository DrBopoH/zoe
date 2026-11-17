package market

import (
	"context"
	"strings"
)

// Экономика США из FRED (тот же публичный CSV без ключа, что и у ставки ФРС): рынок труда, рост, кривая доходности, страх и настроения.
const (
	SourceUSEcon = "usecon"

	SerUSUnrate    = "us_unrate"    // безработица (UNRATE), %, помесячно с 1948 года
	SerUSGDP       = "us_gdp"       // рост реального ВВП (A191RL1Q225SBEA), % в пересчёте на год, поквартально с 1947 года
	SerUSSpread    = "us_spread"    // кривая доходности: 10-летние минус 2-летние Treasury (T10Y2Y), п. п. — последнее значение недели
	SerUSVIX       = "us_vix"       // индекс страха VIX (VIXCLS), пункты — последнее значение недели
	SerUSSentiment = "us_sentiment" // настроения потребителей, университет Мичигана (UMCSENT), индекс, помесячно
)

// USEcon — ряды экономики США. Как у USFRED: сбой одного ряда не убивает остальные, ошибка — в Result.Error.
func (c *Client) USEcon(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceUSEcon}
	r := c.fredAll(ctx, rep, []fredJob{
		{"UNRATE", "безработица", ""}, {"A191RL1Q225SBEA", "рост ВВП", ""}, {"T10Y2Y", "кривая доходности", SerUSSpread}, {"VIXCLS", "индекс страха VIX", SerUSVIX}, {"UMCSENT", "настроения потребителей", ""},
	})
	var out []SeriesPoint
	out = append(out, toPoints(SerUSUnrate, r.Obs["UNRATE"])...)
	out = append(out, toPoints(SerUSGDP, r.Obs["A191RL1Q225SBEA"])...)
	out = append(out, weekly(toPoints(SerUSSpread, r.Obs["T10Y2Y"]))...)
	out = append(out, weekly(toPoints(SerUSVIX, r.Obs["VIXCLS"]))...)
	out = append(out, toPoints(SerUSSentiment, r.Obs["UMCSENT"])...)
	rep.report(95, "")
	res.Fetched, res.Skipped = len(out), r.Skipped
	res.Error, res.Tail = strings.Join(r.Errs, "; "), r.Tail
	return out, res
}

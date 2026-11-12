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
	var out []SeriesPoint
	var errs []string
	fetch := func(id string, pct int, note string) []fredObs {
		rep.report(pct, note)
		obs, sk, err := c.fred(ctx, id)
		res.Skipped += sk
		if err != nil {
			errs = append(errs, err.Error())
			return nil
		}
		return obs
	}
	out = append(out, toPoints(SerUSUnrate, fetch("UNRATE", 10, "безработица"))...)
	out = append(out, toPoints(SerUSGDP, fetch("A191RL1Q225SBEA", 30, "рост ВВП"))...)
	out = append(out, weekly(toPoints(SerUSSpread, fetch("T10Y2Y", 50, "кривая доходности")))...)
	out = append(out, weekly(toPoints(SerUSVIX, fetch("VIXCLS", 75, "индекс страха VIX")))...)
	out = append(out, toPoints(SerUSSentiment, fetch("UMCSENT", 90, "настроения потребителей"))...)
	rep.report(95, "")
	res.Fetched = len(out)
	res.Error = strings.Join(errs, "; ")
	return out, res
}

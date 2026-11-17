package market

import (
	"context"
	"strings"
)

// Кредит и доллар США из FRED: спреды корпоративных облигаций, индекс финансовых условий, широкий индекс доллара и госдолг.
const (
	SourceUSCredit = "uscredit"

	SerUSHY      = "us_hy"       // спред высокодоходных облигаций к Treasury, п. п. (BAMLH0A0HYM2; ICE BofA отдаёт FRED только последние 3 года)
	SerUSIG      = "us_ig"       // спред облигаций инвестиционного рейтинга, п. п. (BAMLC0A0CM; тоже последние 3 года)
	SerUSNFCI    = "us_nfci"     // индекс финансовых условий ФРБ Чикаго (NFCI), еженедельно: выше нуля — условия жёстче среднего
	SerUSUSD     = "us_usd"      // широкий индекс доллара (DTWEXBGS), января 2006 = 100 — последнее значение недели
	SerUSDebtGDP = "us_debt_gdp" // федеральный долг к ВВП, % (GFDEGDQ188S, раз в квартал)
)

// USCredit — ряды кредита и доллара. Сбой одного ряда не убивает остальные, ошибка — в Result.Error.
func (c *Client) USCredit(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceUSCredit}
	r := c.fredAll(ctx, rep, []fredJob{
		{"BAMLH0A0HYM2", "спред HY", SerUSHY}, {"BAMLC0A0CM", "спред IG", SerUSIG}, {"NFCI", "финансовые условия", SerUSNFCI},
		{"DTWEXBGS", "индекс доллара", SerUSUSD}, {"GFDEGDQ188S", "долг к ВВП", SerUSDebtGDP},
	})
	var out []SeriesPoint
	out = append(out, weekly(toPoints(SerUSHY, r.Obs["BAMLH0A0HYM2"]))...)
	out = append(out, weekly(toPoints(SerUSIG, r.Obs["BAMLC0A0CM"]))...)
	out = append(out, toPoints(SerUSNFCI, r.Obs["NFCI"])...)
	out = append(out, weekly(toPoints(SerUSUSD, r.Obs["DTWEXBGS"]))...)
	out = append(out, toPoints(SerUSDebtGDP, r.Obs["GFDEGDQ188S"])...)
	rep.report(95, "")
	res.Fetched, res.Skipped = len(out), r.Skipped
	res.Error, res.Tail = strings.Join(r.Errs, "; "), r.Tail
	return out, res
}

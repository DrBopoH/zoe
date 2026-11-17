package market

import (
	"context"
	"strings"
)

// Цены США из FRED: базовая инфляция, расходы на личное потребление (PCE — любимая мера ФРС), цены производителей и жильё.
// Индексы приходят месячными, наружу уходит изменение к прошлому году и (для базового CPI) к прошлому месяцу.
const (
	SourceUSPrices = "usprices"

	SerUSCoreCPIYoY = "us_corecpi_yoy" // базовый CPI (без еды и энергии) к прошлому году, % (CPILFESL)
	SerUSCoreCPIMoM = "us_corecpi_mom" // базовый CPI к прошлому месяцу, %
	SerUSPCEYoY     = "us_pce_yoy"     // дефлятор PCE к прошлому году, % (PCEPI)
	SerUSCorePCEYoY = "us_corepce_yoy" // базовый PCE к прошлому году, % (PCEPILFE) — по нему ФРС меряет цель в 2 %
	SerUSPPIYoY     = "us_ppi_yoy"     // цены производителей к прошлому году, % (PPIACO, все товары)
	SerUSShelterYoY = "us_shelter_yoy" // жильё в CPI (аренда и условная аренда) к прошлому году, % (CUSR0000SAH1)
)

// USPrices — ряды цен США. Как у USFRED: сбой одного ряда не убивает остальные, ошибка — в Result.Error.
func (c *Client) USPrices(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceUSPrices}
	r := c.fredAll(ctx, rep, []fredJob{
		{"CPILFESL", "базовый CPI", ""}, {"PCEPI", "PCE", ""}, {"PCEPILFE", "базовый PCE", ""}, {"PPIACO", "цены производителей", ""}, {"CUSR0000SAH1", "жильё в CPI", ""},
	})
	var out []SeriesPoint
	yoy, mom := indexChanges(r.Obs["CPILFESL"], SerUSCoreCPIYoY, SerUSCoreCPIMoM)
	out = append(out, yoy...)
	out = append(out, mom...)
	for _, j := range []struct{ id, series string }{{"PCEPI", SerUSPCEYoY}, {"PCEPILFE", SerUSCorePCEYoY}, {"PPIACO", SerUSPPIYoY}, {"CUSR0000SAH1", SerUSShelterYoY}} {
		y, _ := indexChanges(r.Obs[j.id], j.series, "")
		out = append(out, y...)
	}
	rep.report(95, "")
	res.Fetched, res.Skipped = len(out), r.Skipped
	res.Error, res.Tail = strings.Join(r.Errs, "; "), r.Tail
	return out, res
}

package market

import "context"

// Пакеты Норвегии, Швеции и Канады. Источники подтверждены зондами: ставки — BIS WS_CBPOL (NO, SE, CA есть), инфляция Норвегии и Швеции — Евростат HICP
// (prc_hicp_minr, geo NO и SE), безработица — FRED LRHUTTTT (NO, SE, CA, до 2026-08), доходность 10 лет — FRED IRLTLT01 (NO, SE, CA, до 2026-09);
// курсы кроны, кроны и доллара Канады к гривне — НБУ (источник «fx»). Инфляция Канады: Евростат её не отдаёт, FRED-зеркало кончается в 2025 — берём BIS WS_LONG_CPI.
// Не взяли: ВВП, ставки из первички (Norges Bank, Riksbank, Bank of Canada — BIS отдаёт те же решения), нефть и CAD-связки (это M13).
const (
	SourceNO = "no"
	SourceSE = "se"
	SourceCA = "ca"

	SerNOPolicy = "no_policy" // ставка Norges Bank (BIS), % — только даты смены значения
	SerNOCPI    = "no_cpi"    // инфляция HICP г/г, %, помесячно (Евростат)
	SerNOUnrate = "no_u"      // безработица, %, помесячно (FRED, зеркало ОЭСР)
	SerNOY10    = "no_y10"    // доходность 10 лет, %, помесячно (FRED, зеркало ОЭСР)

	SerSEPolicy = "se_policy" // ставка Riksbank (BIS), % — только даты смены значения
	SerSECPI    = "se_cpi"    // инфляция HICP г/г, %, помесячно (Евростат)
	SerSEUnrate = "se_u"      // безработица, %, помесячно (FRED, зеркало ОЭСР)
	SerSEY10    = "se_y10"    // доходность 10 лет, %, помесячно (FRED, зеркало ОЭСР)

	SerCAPolicy = "ca_policy" // ставка Банка Канады (BIS), % — только даты смены значения
	SerCACPI    = "ca_cpi"    // инфляция CPI г/г, % (BIS)
	SerCAUnrate = "ca_u"      // безработица, %, помесячно (FRED, зеркало ОЭСР)
	SerCAY10    = "ca_y10"    // доходность 10 лет, %, помесячно (FRED, зеркало ОЭСР)
)

var (
	noSeries = []string{SerNOPolicy, SerNOCPI, SerNOUnrate, SerNOY10}
	seSeries = []string{SerSEPolicy, SerSECPI, SerSEUnrate, SerSEY10}
	caSeries = []string{SerCAPolicy, SerCACPI, SerCAUnrate, SerCAY10}
)

// Norway — ряды Норвегии: ставка (BIS), инфляция (Евростат), безработица и доходность 10 лет (FRED).
func (c *Client) Norway(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.fredPack(ctx, rep, SourceNO, []fredSeries{
		{"LRHUTTTTNOM156S", SerNOUnrate, "безработица Норвегии"},
		{"IRLTLT01NOM156N", SerNOY10, "доходность 10 лет Норвегии"},
	}, c.countryJobs("NO", SerNOPolicy, SerNOCPI))
}

// Sweden — ряды Швеции: ставка (BIS), инфляция (Евростат), безработица и доходность 10 лет (FRED).
func (c *Client) Sweden(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.fredPack(ctx, rep, SourceSE, []fredSeries{
		{"LRHUTTTTSEM156S", SerSEUnrate, "безработица Швеции"},
		{"IRLTLT01SEM156N", SerSEY10, "доходность 10 лет Швеции"},
	}, c.countryJobs("SE", SerSEPolicy, SerSECPI))
}

// Canada — ряды Канады: ставка и инфляция (BIS), безработица и доходность 10 лет (FRED).
func (c *Client) Canada(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	var jobs []packJob
	if c.BISBase != "" {
		jobs = append(jobs, bisPolicy(SerCAPolicy, "CA"), bisCPIJob(SerCACPI, "CA"))
	}
	return c.fredPack(ctx, rep, SourceCA, []fredSeries{
		{"LRHUTTTTCAM156S", SerCAUnrate, "безработица Канады"},
		{"IRLTLT01CAM156N", SerCAY10, "доходность 10 лет Канады"},
	}, jobs)
}

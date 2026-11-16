package market

import "context"

// Пакеты Австралии и Бразилии. Источники подтверждены зондом Дока 16.11.2026 (probe5).
// BIS: ставка (WS_CBPOL), инфляция (WS_LONG_CPI), реальный курс (общий список reer.go). Индексы ASX 200 и Bovespa — Yahoo (krtw.go, среднее 2015 = 100).
// Доходность 10 лет и безработица Австралии — FRED, зеркало ОЭСР (зондом не проверялись: FRED из зонда не отвечает). Доходности Бразилии в FRED нет (IRLTLT01BRM156N — HTTP 404), безработицы Бразилии тоже не берём.
// Курс к гривне: у НБУ есть австралийский доллар (общий источник курсов), бразильского реала в его списке нет.
const (
	SourceAU = "au"
	SourceBR = "br"

	SerAUPolicy = "au_policy" // ставка Резервного банка Австралии (BIS), % — только даты смены значения
	SerAUCPI    = "au_cpi"    // инфляция CPI г/г, % (BIS)
	SerAUUnrate = "au_u"      // безработица, %, помесячно (FRED, зеркало ОЭСР)
	SerAUY10    = "au_y10"    // доходность 10 лет, %, помесячно (FRED, зеркало ОЭСР)
	SerBRPolicy = "br_policy" // ставка Selic (BIS), % — только даты смены значения
	SerBRCPI    = "br_cpi"    // инфляция CPI г/г, % (BIS)
)

var (
	auSeries = []string{SerAUPolicy, SerAUCPI, SerAUUnrate, SerAUY10, SerWM("au")}
	brSeries = []string{SerBRPolicy, SerBRCPI, SerWM("br")}
)

// Australia — ряды Австралии. Сбой части (FRED, BIS, Yahoo) и отдельного ряда не трогает остальное.
func (c *Client) Australia(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.countryPack(ctx, rep, SourceAU, "^AXJO", SerWM("au"), []fredSeries{
		{"LRUNTTTTAUM156S", SerAUUnrate, "безработица Австралии"},
		{"IRLTLT01AUM156N", SerAUY10, "доходность 10 лет Австралии"},
	}, bisPolicy(SerAUPolicy, "AU"), bisCPIJob(SerAUCPI, "AU"))
}

// Brazil — ряды Бразилии.
func (c *Client) Brazil(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.countryPack(ctx, rep, SourceBR, "^BVSP", SerWM("br"), nil, bisPolicy(SerBRPolicy, "BR"), bisCPIJob(SerBRCPI, "BR"))
}

// countryPack — пакет страны из трёх частей: FRED, BIS (ставка, инфляция) и индекс биржи с Yahoo. Часть без адреса источника пропускается.
func (c *Client) countryPack(ctx context.Context, rep Reporter, source, symbol, indexSeries string, fs []fredSeries, bis ...packJob) ([]SeriesPoint, Result) {
	if c.BISBase == "" {
		bis = nil
	}
	out, res := c.fredPack(ctx, rep, source, fs, bis)
	if c.YahooBase != "" {
		rep.report(95, symbol)
		res = mergeResult(res, c.yahooJob(ctx, symbol, indexSeries, &out))
	}
	res.Fetched = len(out)
	return out, res
}

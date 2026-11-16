package market

import "context"

// Пакеты Саудовской Аравии, ОАЭ и Ирана. Источники подтверждены зондом Дока 17.11.2026 (probe6).
// Саудовская Аравия: BIS — ставка (репо САМА), инфляция, реальный курс; Yahoo — индекс Тадавул (^TASI.SR); у НБУ есть саудовский риял.
// ОАЭ: у BIS нет ставки (дирхам привязан к доллару), инфляция там кончается на конец 2025 года — показываем что есть; Yahoo — индекс Дубайской биржи (DFMGI.AE, индекс Абу-Даби на Yahoo не найден); у НБУ есть дирхам.
// Иран: у BIS нет ни ставки, ни инфляции, НБУ не котирует риал, IMF отвечает отказом (403), индекса Тегерана на Yahoo нет —
// остаётся годовая инфляция Всемирного банка (как у Египта). PPP Ирана в набор не берём: официальный курс риала далёк от рыночного, ряд бы только вводил в заблуждение.
const (
	SourceSA = "sa"
	SourceAE = "ae"
	SourceIR = "ir"

	SerSAPolicy = "sa_policy" // репо-ставка САМА (BIS), % — только даты смены значения
	SerSACPI    = "sa_cpi"    // инфляция CPI г/г, % (BIS)
	SerAECPI    = "ae_cpi"    // инфляция CPI г/г, % (BIS, данные до конца 2025)
	SerIRCPI    = "ir_cpi"    // инфляция потребительских цен, % за год (Всемирный банк, FP.CPI.TOTL.ZG)
)

var (
	saSeries = []string{SerSAPolicy, SerSACPI, SerWM("sa")}
	aeSeries = []string{SerAECPI, SerWM("ae")}
	irSeries = []string{SerIRCPI}
)

// SaudiArabia — ряды Саудовской Аравии.
func (c *Client) SaudiArabia(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.countryPack(ctx, rep, SourceSA, "^TASI.SR", SerWM("sa"), nil, bisPolicy(SerSAPolicy, "SA"), bisCPIJob(SerSACPI, "SA"))
}

// UAE — ряды ОАЭ.
func (c *Client) UAE(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.countryPack(ctx, rep, SourceAE, "DFMGI.AE", SerWM("ae"), nil, bisCPIJob(SerAECPI, "AE"))
}

// Iran — ряды Ирана: годовая инфляция Всемирного банка.
func (c *Client) Iran(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	if c.WBBase == "" {
		return nil, Result{Source: SourceIR}
	}
	return c.runJobs(ctx, rep, SourceIR, []packJob{wbJob(SerIRCPI, "IRN", "FP.CPI.TOTL.ZG")})
}

package market

import "context"

// Пакеты России и Египта. Из Украины недоступны сами центробанки и статистика (ЦБ РФ, Росстат, MOEX, ЦБ Египта — таймаут или блок). Россия: ставка и инфляция
// через BIS (WS_CBPOL и WS_LONG_CPI, помесячно, с лагом). Египта в BIS нет (HTTP 404, проверено на живых данных): инфляция — годовая, из Всемирного банка.
// Курсы рубля и египетского фунта к гривне — из НБУ (источник «fx»).
// Не взяли: Россия — доходность ОФЗ, IMOEX, резервы; Египет — ставка ЦБ, помесячная инфляция, резервы, ВВП, параллельный курс (нет открытого источника из Украины).
// Инфляция считается устойчиво к высоким значениям: годовое изменение из индекса, без допущений о порядке величины.
const (
	SourceRU = "ru"
	SourceEG = "eg"

	SerRUPolicy = "ru_policy" // ключевая ставка Банка России (BIS), % — только даты смены значения
	SerRUCPI    = "ru_cpi"    // инфляция г/г, % (BIS)
	SerEGCPI    = "eg_cpi"    // инфляция потребительских цен, % за год (Всемирный банк, FP.CPI.TOTL.ZG), точка на конец года
)

var (
	ruSeries = []string{SerRUPolicy, SerRUCPI}
	egSeries = []string{SerEGCPI}
)

func (c *Client) bisPack(ctx context.Context, rep Reporter, source, iso2, policyID, cpiID string) ([]SeriesPoint, Result) {
	if c.BISBase == "" {
		return nil, Result{Source: source}
	}
	return c.runJobs(ctx, rep, source, []packJob{bisPolicy(policyID, iso2), bisCPIJob(cpiID, iso2)})
}

// Russia — ряды России: ставка и инфляция из BIS.
func (c *Client) Russia(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.bisPack(ctx, rep, SourceRU, "RU", SerRUPolicy, SerRUCPI)
}

// Egypt — ряды Египта: годовая инфляция из Всемирного банка.
func (c *Client) Egypt(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	if c.WBBase == "" {
		return nil, Result{Source: SourceEG}
	}
	return c.runJobs(ctx, rep, SourceEG, []packJob{wbJob(SerEGCPI, "EGY", "FP.CPI.TOTL.ZG")})
}

package market

import "context"

// Пакеты России и Египта. Из Украины недоступны сами центробанки и статистика (ЦБ РФ, Росстат, MOEX, ЦБ Египта — таймаут или блок), поэтому ставка и инфляция идут
// через BIS (WS_CBPOL и WS_LONG_CPI, помесячно, с лагом), а курсы рубля и египетского фунта к гривне — из НБУ (источник «fx»).
// Не взяли: доходность ОФЗ, IMOEX, резервы, ВВП, параллельный курс египетского фунта — нет открытого источника из Украины.
// Инфляция считается устойчиво к высоким значениям: годовое изменение из индекса, без допущений о порядке величины.
const (
	SourceRU = "ru"
	SourceEG = "eg"

	SerRUPolicy = "ru_policy" // ключевая ставка Банка России (BIS), % — только даты смены значения
	SerRUCPI    = "ru_cpi"    // инфляция г/г, % (BIS)
	SerEGPolicy = "eg_policy" // ставка Центрального банка Египта (BIS), % — только даты смены значения
	SerEGCPI    = "eg_cpi"    // инфляция г/г, % (BIS)
)

var (
	ruSeries = []string{SerRUPolicy, SerRUCPI}
	egSeries = []string{SerEGPolicy, SerEGCPI}
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

// Egypt — ряды Египта: ставка и инфляция из BIS.
func (c *Client) Egypt(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.bisPack(ctx, rep, SourceEG, "EG", SerEGPolicy, SerEGCPI)
}

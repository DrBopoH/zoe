package market

import "context"

// Пакет Индии. Источники подтверждены зондом Дока 17.11.2026 (probe7): BIS — ставка (репо РБИ), инфляция, реальный курс;
// Yahoo — индекс BSE Sensex (^BSESN, с 1997 года; Nifty 50 короче — с 2007); у НБУ есть индийская рупия; Всемирный банк — PPP.
const (
	SourceIN = "in"

	SerINPolicy = "in_policy" // репо-ставка Резервного банка Индии (BIS), % — только даты смены значения
	SerINCPI    = "in_cpi"    // инфляция CPI г/г, % (BIS)
)

var inSeries = []string{SerINPolicy, SerINCPI, SerWM("in")}

// India — ряды Индии: ставка и инфляция из BIS, индекс Sensex с Yahoo.
func (c *Client) India(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.countryPack(ctx, rep, SourceIN, "^BSESN", SerWM("in"), nil, bisPolicy(SerINPolicy, "IN"), bisCPIJob(SerINCPI, "IN"))
}

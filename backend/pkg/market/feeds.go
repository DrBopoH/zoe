package market

import "context"

// Feed — один источник рыночных данных как самостоятельный процесс: свой id, своё название, свой итог.
// Новый источник (ключевая ставка, депозитные сертификаты НБУ, ФРС, инфляция) — ещё одна запись в Feeds,
// без новых маршрутов и без переделки запуска и тостов.
//
// Источник облигаций заполняет Fetch, источник числовых рядов (ставки, инфляция) — Series; нужно ровно одно из двух.
// Unit — что считать в итоге («бумаг», «точек»): попадает в окно процесса и в тост.
type Feed struct {
	ID     string
	Title  string
	Unit   string
	Fetch  func(ctx context.Context, rep Reporter) ([]Security, Result)
	Series func(ctx context.Context, rep Reporter) ([]SeriesPoint, Result)
}

// IsSource — известен ли такой источник (для ?source=… в маршрутах).
func IsSource(id string) bool {
	switch id {
	case SourceNBU, SourceUST, SourceMacro, SourceFX, SourceUS, SourceUSEcon, SourceCrypto:
		return true
	}
	return false
}

// Reporter — ход долгого источника: процент 0..100 и короткая заметка («аукционы: 10.11»). nil допустим.
// Фронт рисует это в окне процесса; источник без процента остаётся бегущей полосой.
type Reporter func(pct int, note string)

func (r Reporter) report(pct int, note string) {
	if r != nil {
		r(pct, note)
	}
}

// Feeds — все источники клиента в порядке показа.
func (c *Client) Feeds() []Feed {
	return []Feed{
		{ID: SourceNBU, Title: "ОВДП · НБУ", Unit: "бумаг", Fetch: c.SyncNBU},
		{ID: SourceUST, Title: "Облигации США", Unit: "бумаг", Fetch: func(ctx context.Context, rep Reporter) ([]Security, Result) {
			rep.report(0, "FiscalData")
			list, res, err := c.USTAuctions(ctx)
			if err != nil {
				res.Error = err.Error()
			}
			return list, res
		}},
		{ID: SourceMacro, Title: "Ставки и инфляция · НБУ", Unit: "точек", Series: c.MacroNBU},
		{ID: SourceFX, Title: "Курсы валют и золото · НБУ", Unit: "точек", Series: c.FXNBU},
		{ID: SourceUS, Title: "Ставка ФРС и инфляция США · FRED", Unit: "точек", Series: c.USFRED},
		{ID: SourceUSEcon, Title: "Экономика США · FRED", Unit: "точек", Series: c.USEcon},
		{ID: SourceCrypto, Title: "Крипто · Binance", Unit: "точек", Series: c.Crypto},
	}
}

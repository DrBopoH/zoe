package market

import "context"

// Feed — один источник рыночных данных как самостоятельный процесс: свой id, своё название, свой итог.
// Новый источник (ключевая ставка, депозитные сертификаты НБУ, ФРС, инфляция) — ещё одна запись в Feeds,
// без новых маршрутов и без переделки запуска и тостов.
type Feed struct {
	ID    string
	Title string
	Fetch func(ctx context.Context, rep Reporter) ([]Security, Result)
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
		{ID: SourceNBU, Title: "ОВДП · НБУ", Fetch: c.SyncNBU},
		{ID: SourceUST, Title: "Облигации США", Fetch: func(ctx context.Context, rep Reporter) ([]Security, Result) {
			rep.report(0, "FiscalData")
			list, res, err := c.USTAuctions(ctx)
			if err != nil {
				res.Error = err.Error()
			}
			return list, res
		}},
	}
}

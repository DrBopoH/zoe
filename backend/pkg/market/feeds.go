package market

import "context"

// Feed — один источник рыночных данных как самостоятельный процесс: свой id, своё название, свой итог.
// Новый источник (ключевая ставка, депозитные сертификаты НБУ, ФРС, инфляция) — ещё одна запись в Feeds,
// без новых маршрутов и без переделки запуска и тостов.
//
// Источник облигаций заполняет Fetch, источник числовых рядов (ставки, инфляция) — Series; нужно ровно одно из двух.
// Unit — что считать в итоге («бумаг», «точек»): попадает в окно процесса и в тост.
// Provides — id рядов, которые источник кладёт в кеш (только для Series). Это единственное место, где перечислено, что откуда берётся:
// фронт спрашивает список у сервера (GET /api/macro/catalog), а не держит свои копии.
type Feed struct {
	ID       string
	Title    string
	Unit     string
	Provides []string
	Fetch    func(ctx context.Context, rep Reporter) ([]Security, Result)
	Series   func(ctx context.Context, rep Reporter) ([]SeriesPoint, Result)
}

// Catalog — какие ряды даёт каждый числовой источник: источник → id рядов, в порядке показа.
func (c *Client) Catalog() map[string][]string {
	out := map[string][]string{}
	for _, f := range c.Feeds() {
		if f.Provides != nil {
			out[f.ID] = f.Provides
		}
	}
	return out
}

// IsSource — известен ли такой источник (для ?source=… в маршрутах).
func IsSource(id string) bool {
	switch id {
	case SourceNBU, SourceUST, SourceMacro, SourceFX, SourceUS, SourceUSEcon, SourceUSCurve, SourceUSReal, SourceUSPrices, SourceUSJobs, SourceUSAct, SourceUSLiq, SourceUSCredit, SourceEZ, SourceGB, SourceCH, SourceJP, SourceCN, SourceRU, SourceEG, SourcePL, SourceTR, SourceNO, SourceSE, SourceCA, SourceCrypto, SourceStocks:
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
		{ID: SourceMacro, Title: "Ставки и инфляция · НБУ, Евростат", Unit: "точек", Provides: []string{SerPolicyRate, SerOvernightCD, SerOvernightLoans, SerCPIYoY, SerCPIMoM, SerCoreYoY, SerCoreMoM, SerEUCPIYoY}, Series: c.MacroNBU},
		{ID: SourceFX, Title: "Курсы валют и золото · НБУ", Unit: "точек", Provides: []string{SerUSDUAH, SerEURUAH, SerXAUUAH, SerGBPUAH, SerCHFUAH, SerJPYUAH, SerCNYUAH, SerRUBUAH, SerEGPUAH, SerPLNUAH, SerTRYUAH, SerNOKUAH, SerSEKUAH, SerCADUAH}, Series: c.FXNBU},
		{ID: SourceUS, Title: "Ставка ФРС и инфляция США · FRED", Unit: "точек", Provides: []string{SerFedFunds, SerFedTarget, SerUSCPIYoY, SerUSCPIMoM, SerUST10Y}, Series: c.USFRED},
		{ID: SourceUSEcon, Title: "Экономика США · FRED", Unit: "точек", Provides: []string{SerUSUnrate, SerUSGDP, SerUSSpread, SerUSVIX, SerUSSentiment}, Series: c.USEcon},
		{ID: SourceUSCurve, Title: "Кривая доходности США · FRED", Unit: "точек", Provides: []string{SerUST1M, SerUST3M, SerUST6M, SerUST1Y, SerUST2Y, SerUST3Y, SerUST5Y, SerUST7Y, SerUST20Y, SerUST30Y, SerUSSpread3M}, Series: c.USCurve},
		{ID: SourceUSReal, Title: "Реальные ставки и ожидаемая инфляция США · FRED", Unit: "точек", Provides: []string{SerUSReal5, SerUSReal10, SerUSBE5, SerUSBE10, SerUSFwd5y5y, SerUSTermPrem, SerUSMortgage}, Series: c.USReal},
		{ID: SourceUSPrices, Title: "Цены США · FRED", Unit: "точек", Provides: []string{SerUSCoreCPIYoY, SerUSCoreCPIMoM, SerUSPCEYoY, SerUSCorePCEYoY, SerUSPPIYoY, SerUSShelterYoY}, Series: c.USPrices},
		{ID: SourceUSJobs, Title: "Рынок труда США · FRED", Unit: "точек", Provides: []string{SerUSNFP, SerUSClaims, SerUSJOLTS, SerUSWages, SerUSU6, SerUSECIYoY}, Series: c.USJobs},
		{ID: SourceUSAct, Title: "Активность США · FRED", Unit: "точек", Provides: []string{SerUSRetailYoY, SerUSIndproYoY, SerUSHousing, SerUSHomePriceYoY}, Series: c.USActivity},
		{ID: SourceUSLiq, Title: "Ликвидность США · FRED", Unit: "точек", Provides: []string{SerUSM2, SerUSM2YoY, SerUSFedBS, SerUSRRP, SerUSTGA, SerUSNetLiq, SerUSSOFR}, Series: c.USLiquidity},
		{ID: SourceUSCredit, Title: "Кредит и доллар США · FRED", Unit: "точек", Provides: []string{SerUSHY, SerUSIG, SerUSNFCI, SerUSUSD, SerUSDebtGDP}, Series: c.USCredit},
		{ID: SourceEZ, Title: "Еврозона · ЕЦБ и Евростат", Unit: "точек", Provides: append([]string{SerEZPolicy, SerEZMRO, SerEZESTR, SerEZY2, SerEZY10, SerEZCPICore, SerEZUnrate, SerEZGDP}, ezMemberSeries()...), Series: c.Eurozone},
		{ID: SourceGB, Title: "Великобритания · Банк Англии и ONS", Unit: "точек", Provides: gbSeries, Series: c.UK},
		{ID: SourceCH, Title: "Швейцария · BIS и Евростат", Unit: "точек", Provides: chSeries, Series: c.Switzerland},
		{ID: SourceJP, Title: "Япония · BIS и FRED", Unit: "точек", Provides: jpSeries, Series: c.Japan},
		{ID: SourceCN, Title: "Китай · BIS", Unit: "точек", Provides: cnSeries, Series: c.China},
		{ID: SourceRU, Title: "Россия · BIS", Unit: "точек", Provides: ruSeries, Series: c.Russia},
		{ID: SourceEG, Title: "Египет · Всемирный банк", Unit: "точек", Provides: egSeries, Series: c.Egypt},
		{ID: SourcePL, Title: "Польша · BIS, Евростат и FRED", Unit: "точек", Provides: plSeries, Series: c.Poland},
		{ID: SourceTR, Title: "Турция · BIS, Евростат и FRED", Unit: "точек", Provides: trSeries, Series: c.Turkey},
		{ID: SourceNO, Title: "Норвегия · BIS, Евростат и FRED", Unit: "точек", Provides: noSeries, Series: c.Norway},
		{ID: SourceSE, Title: "Швеция · BIS, Евростат и FRED", Unit: "точек", Provides: seSeries, Series: c.Sweden},
		{ID: SourceCA, Title: "Канада · BIS и FRED", Unit: "точек", Provides: caSeries, Series: c.Canada},
		{ID: SourceCrypto, Title: "Крипто · Binance", Unit: "точек", Provides: []string{SerCryptoBTC, SerCryptoETH, SerCryptoBNB, SerCryptoSOL, SerCryptoFNG}, Series: c.Crypto},
		{ID: SourceStocks, Title: "Биржа США · FRED", Unit: "точек", Provides: []string{SerStockSP500, SerStockNasdaq, SerStockDow}, Series: c.Stocks},
	}
}

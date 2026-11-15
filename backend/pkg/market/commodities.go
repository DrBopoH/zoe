package market

import (
	"context"
	"strings"
)

// Сырьё из FRED (тот же публичный CSV без ключа). Все идентификаторы подтверждены зондом 16.11.2026 (probe6-comm):
// нефть и газ США — ежедневно (EIA), остальное — помесячно (Всемирный банк, «цены на сырьё»). Единицы цен в долларах США.
// Металлы золото, серебро, платина и палладий приходят из НБУ (источник «fx», гривен за унцию) — здесь их нет.
// Не взяли: золото, серебро, платину из FRED (отдаёт защитную страницу вместо данных, а у НБУ они есть).
const (
	SourceCommodities = "commodities"

	SerComBrent  = "com_brent"  // нефть Brent, $/баррель, ежедневно
	SerComWTI    = "com_wti"    // нефть WTI, $/баррель, ежедневно
	SerComGasUS  = "com_gas_us" // газ Henry Hub, $/MMBtu, ежедневно
	SerComGasEU  = "com_gas_eu" // газ Европа (TTF), $/MMBtu, помесячно
	SerComCopper = "com_copper" // медь, $/т, помесячно
	SerComAlum   = "com_alum"   // алюминий, $/т
	SerComNickel = "com_nickel" // никель, $/т
	SerComWheat  = "com_wheat"  // пшеница, $/т
	SerComCorn   = "com_corn"   // кукуруза, $/т
	SerComRice   = "com_rice"   // рис (Таиланд 5 %), $/т
	SerComSoy    = "com_soy"    // соя, $/т
	SerComSugar  = "com_sugar"  // сахар (№11), цент/фунт
	SerComCoffee = "com_coffee" // кофе арабика, цент/фунт
	SerComCocoa  = "com_cocoa"  // какао, $/т
)

var commoditySeries = []string{
	SerComBrent, SerComWTI, SerComGasUS, SerComGasEU,
	SerComCopper, SerComAlum, SerComNickel,
	SerComWheat, SerComCorn, SerComRice, SerComSoy, SerComSugar, SerComCoffee, SerComCocoa,
}

var commodityJobs = []struct{ fred, series, note string }{
	{"DCOILBRENTEU", SerComBrent, "нефть Brent"},
	{"DCOILWTICO", SerComWTI, "нефть WTI"},
	{"DHHNGSP", SerComGasUS, "газ Henry Hub"},
	{"PNGASEUUSDM", SerComGasEU, "газ Европа"},
	{"PCOPPUSDM", SerComCopper, "медь"},
	{"PALUMUSDM", SerComAlum, "алюминий"},
	{"PNICKUSDM", SerComNickel, "никель"},
	{"PWHEAMTUSDM", SerComWheat, "пшеница"},
	{"PMAIZMTUSDM", SerComCorn, "кукуруза"},
	{"PRICENPQUSDM", SerComRice, "рис"},
	{"PSOYBUSDM", SerComSoy, "соя"},
	{"PSUGAISAUSDM", SerComSugar, "сахар"},
	{"PCOFFOTMUSDM", SerComCoffee, "кофе"},
	{"PCOCOUSDM", SerComCocoa, "какао"},
}

// Commodities — нефть, газ, промышленные металлы и сельхозтовары. Сбой одного ряда не трогает остальные, ошибка — в Result.Error.
func (c *Client) Commodities(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceCommodities}
	if c.FREDBase == "" {
		return nil, res
	}
	jobs := make([]fredJob, len(commodityJobs))
	for i, j := range commodityJobs {
		jobs[i] = fredJob{ID: j.fred, Note: j.note, Series: j.series}
	}
	r := c.fredAll(ctx, rep, jobs)
	var out []SeriesPoint
	for _, j := range commodityJobs {
		out = append(out, toPoints(j.series, r.Obs[j.fred])...)
	}
	res.Fetched, res.Skipped, res.Tail, res.Error = len(out), r.Skipped, r.Tail, strings.Join(r.Errs, "; ")
	return out, res
}

package market

import (
	"context"
	"strings"
)

// Еврозона: ставки и рынок ЕЦБ (SDMX csvdata) и экономика Евростата (SDMX-CSV), без ключей. Общий инфляционный HICP еврозоны (eu_cpi_yoy) лежит
// в источнике «macro» — здесь только то, чего там нет. Страны-члены (Германия, Франция, …) и их спреды к Bund — ezm.go. Имена рядов — по правилу <страна>_<показатель>.
const (
	SourceEZ = "ez"

	SerEZPolicy  = "ez_policy"   // ставка по депозитам ЕЦБ (DFR), % — только даты смены значения
	SerEZMRO     = "ez_mro"      // основная ставка рефинансирования ЕЦБ (MRO), % — только даты смены
	SerEZESTR    = "ez_estr"     // €STR — ставка овернайт без обеспечения, % — последнее значение недели
	SerEZY2      = "ez_y2"       // доходность 2-летних гособлигаций еврозоны (кривая ЕЦБ, рейтинг AAA), % — последнее значение недели
	SerEZY10     = "ez_y10"      // доходность 10-летних (кривая ЕЦБ, рейтинг AAA — это не Bund, а корзина лучших стран), % — последнее значение недели
	SerEZCPICore = "ez_cpi_core" // базовая инфляция еврозоны (HICP без энергии и еды) к прошлому году, %
	SerEZUnrate  = "ez_u"        // безработица еврозоны (с учётом сезонности), %, помесячно
	SerEZGDP     = "ez_gdp"      // рост реального ВВП еврозоны к предыдущему кварталу, %, поквартально
)

// У Евростата код еврозоны меняется при вступлении новой страны (EA19 → EA20 → EA21): пробуем по очереди, пока не придут данные.
var ezGeos = []string{"EA21", "EA20", "EA"}

// packJob — один ряд пакета: откуда взять (url), как разобрать (get; пусто — SDMX-CSV) и как обработать (post). Общий для еврозоны и пакетов стран (pack.go).
type packJob struct {
	series string
	url    func(c *Client, geo string) string
	geos   bool // перебирать коды еврозоны (иначе url не зависит от geo)
	get    func(c *Client, ctx context.Context, url, series string) ([]SeriesPoint, error)
	post   func([]SeriesPoint) []SeriesPoint
}

func ecbURL(c *Client, flowKey string) string {
	return strings.TrimRight(c.ECBBase, "/") + "/service/data/" + flowKey + "?format=csvdata"
}

func eurostatURL(c *Client, flowKey string) string {
	return strings.TrimRight(c.EurostatBase, "/") + "/eurostat/api/dissemination/sdmx/2.1/data/" + flowKey + "?format=SDMX-CSV"
}

func retag(id string, post func([]SeriesPoint) []SeriesPoint) func([]SeriesPoint) []SeriesPoint {
	return func(pts []SeriesPoint) []SeriesPoint {
		out := post(pts)
		for i := range out {
			out[i].Series = id
		}
		return out
	}
}

func ident(p []SeriesPoint) []SeriesPoint { return p }

// Eurozone — ряды еврозоны. Сбой одного ряда не убивает остальные, ошибка — в Result.Error.
func (c *Client) Eurozone(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	ecb := func(id, key string, post func([]SeriesPoint) []SeriesPoint) packJob {
		return packJob{series: id, url: func(c *Client, _ string) string { return ecbURL(c, key) }, post: retag(id, post)}
	}
	jobs := []packJob{
		ecb(SerEZPolicy, "FM/D.U2.EUR.4F.KR.DFR.LEV", changePoints),
		ecb(SerEZMRO, "FM/D.U2.EUR.4F.KR.MRR_FR.LEV", changePoints),
		ecb(SerEZESTR, "EST/B.EU000A2X2A25.WT", weekly),
		ecb(SerEZY2, "YC/B.U2.EUR.4F.G_N_A.SV_C_YM.SR_2Y", weekly),
		ecb(SerEZY10, "YC/B.U2.EUR.4F.G_N_A.SV_C_YM.SR_10Y", weekly),
	}
	if c.ECBBase == "" {
		jobs = nil
	}
	if c.EurostatBase != "" {
		jobs = append(jobs,
			packJob{series: SerEZCPICore, url: func(c *Client, _ string) string { return eurostatURL(c, "prc_hicp_minr/M.RCH_A.TOT_X_NRG_FOOD.EA") }, post: ident},
			packJob{series: SerEZUnrate, geos: true, url: func(c *Client, geo string) string { return eurostatURL(c, "une_rt_m/M.SA.TOTAL.PC_ACT.T."+geo) }, post: ident},
			packJob{series: SerEZGDP, geos: true, url: func(c *Client, geo string) string { return eurostatURL(c, "namq_10_gdp/Q.CLV_PCH_PRE.SCA.B1GQ."+geo) }, post: ident},
		)
	}
	if c.EurostatBase != "" {
		jobs = append(jobs, memberJobs()...)
	}
	out, res := c.runJobs(ctx, rep, SourceEZ, jobs)
	out = append(out, memberSpreads(groupSeries(out))...)
	res.Fetched = len(out)
	return out, res
}

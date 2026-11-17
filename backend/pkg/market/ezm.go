package market

import (
	"math"
	"sort"
	"strings"
)

// Страны еврозоны из Евростата (тот же источник «ez», что и еврозона целиком): инфляция HICP, безработица и доходность 10 лет по каждой.
// Новая страна = одна запись в ezMembers (плюс запись в реестре стран country.go). Имена рядов — <cc>_cpi, <cc>_u, <cc>_y10.
var ezMembers = []string{"de", "fr", "it", "es", "nl"}

// Спреды к Германии: доходность 10 лет страны минус доходность Bund, п. п. — цена, которую рынок просит за риск страны внутри валютного союза.
var ezSpreadOf = []string{"it", "fr", "es"}

const ezBund = "de"

func memberID(cc, part string) string { return cc + "_" + part }
func spreadID(cc string) string       { return cc + "_bund_spread" }

// ezMemberSeries — все ряды стран-членов: по три на страну и спреды.
func ezMemberSeries() []string {
	var ids []string
	for _, cc := range ezMembers {
		ids = append(ids, memberID(cc, "cpi"), memberID(cc, "u"), memberID(cc, "y10"))
	}
	for _, cc := range ezSpreadOf {
		ids = append(ids, spreadID(cc))
	}
	return ids
}

func memberJobs() []packJob {
	var jobs []packJob
	for _, cc := range ezMembers {
		geo := strings.ToUpper(cc)
		jobs = append(jobs,
			packJob{series: memberID(cc, "cpi"), url: func(c *Client, _ string) string { return eurostatURL(c, "prc_hicp_minr/M.RCH_A.TOTAL."+geo) }, post: ident},
			packJob{series: memberID(cc, "u"), url: func(c *Client, _ string) string { return eurostatURL(c, "une_rt_m/M.SA.TOTAL.PC_ACT.T."+geo) }, post: ident},
			packJob{series: memberID(cc, "y10"), url: func(c *Client, _ string) string { return eurostatURL(c, "irt_lt_mcby_m/M.MCBY."+geo) }, post: ident},
		)
	}
	return jobs
}

// memberSpreads — разность доходностей страны и Германии по совпадающим датам; нет пары — нет ряда (сбой одной страны не рушит остальные).
func memberSpreads(by map[string][]SeriesPoint) []SeriesPoint {
	bund := map[string]float64{}
	for _, p := range by[memberID(ezBund, "y10")] {
		bund[p.Date] = p.Value
	}
	var out []SeriesPoint
	for _, cc := range ezSpreadOf {
		var part []SeriesPoint
		for _, p := range by[memberID(cc, "y10")] {
			if b, ok := bund[p.Date]; ok {
				part = append(part, SeriesPoint{Series: spreadID(cc), Date: p.Date, Value: round2(p.Value - b)})
			}
		}
		sort.Slice(part, func(i, j int) bool { return part[i].Date < part[j].Date })
		out = append(out, part...)
	}
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// groupSeries раскладывает точки по рядам (id → точки).
func groupSeries(pts []SeriesPoint) map[string][]SeriesPoint {
	m := map[string][]SeriesPoint{}
	for _, p := range pts {
		m[p.Series] = append(m[p.Series], p)
	}
	return m
}

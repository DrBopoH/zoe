package market

import (
	"sort"
	"time"
)

// Fact — значение показателя компании на дату из отчётности (число акций, выручка, …); Tag — откуда (у акций баланс главнее обложки), Period — кадр SEC.
type Fact struct {
	Tag    string
	Period string
	End    string
	Val    float64
}

// CapPoint — рыночная капитализация на конец месяца, $.
type CapPoint struct {
	Date string  `json:"d"`
	Cap  float64 `json:"v"`
}

const shareFactMaxAge = 460 * 24 * time.Hour // число акций старше 15 месяцев к месяцу не применяется: между отчётами компания могла измениться

// CapSeries — капитализация по месяцам: закрытие месяца × акции из последнего отчёта не позже конца месяца.
// Акции в отчётности даны «как тогда», цена Yahoo поправлена на все сплиты до сегодня, поэтому число акций умножается на сплиты ПОСЛЕ даты отчёта.
// Месяц без подходящего отчёта пропускается: лучше пробел, чем неверное число. Если за один период есть и баланс, и обложка — берётся баланс.
func CapSeries(bars []PriceBar, splits []Split, shares []Fact) []CapPoint {
	byPeriod := map[string]Fact{}
	for _, f := range shares {
		if f.Val <= 0 || f.End == "" {
			continue
		}
		if old, ok := byPeriod[f.Period]; ok && (old.Tag != TagSharesCover || f.Tag != TagSharesBalance) {
			continue
		}
		byPeriod[f.Period] = f
	}
	facts := make([]Fact, 0, len(byPeriod))
	for _, f := range byPeriod {
		facts = append(facts, f)
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].End != facts[j].End {
			return facts[i].End < facts[j].End
		}
		return facts[i].Period < facts[j].Period
	})
	var out []CapPoint
	for _, b := range bars {
		first, err := time.Parse("2006-01-02", b.Date)
		if err != nil {
			continue
		}
		monthEnd := first.AddDate(0, 1, -1)
		oldest := monthEnd.Add(-shareFactMaxAge).Format("2006-01-02")
		end := monthEnd.Format("2006-01-02")
		var pick *Fact
		for i := len(facts) - 1; i >= 0; i-- {
			if facts[i].End <= end {
				pick = &facts[i]
				break
			}
		}
		if pick == nil || pick.End < oldest {
			continue
		}
		factor := 1.0
		for _, s := range splits {
			if s.Date > pick.End {
				factor *= s.Ratio
			}
		}
		out = append(out, CapPoint{Date: b.Date, Cap: b.Close * pick.Val * factor})
	}
	return out
}

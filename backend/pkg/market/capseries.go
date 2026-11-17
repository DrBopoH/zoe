package market

import (
	"math"
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
	base := sharesToday(facts, splits)
	var out []CapPoint
	for _, b := range bars {
		first, err := time.Parse("2006-01-02", b.Date)
		if err != nil {
			continue
		}
		monthEnd := first.AddDate(0, 1, -1)
		oldest := monthEnd.Add(-shareFactMaxAge).Format("2006-01-02")
		end := monthEnd.Format("2006-01-02")
		pickAt := -1
		for i := len(facts) - 1; i >= 0; i-- {
			if facts[i].End <= end {
				pickAt = i
				break
			}
		}
		if pickAt < 0 || facts[pickAt].End < oldest {
			continue
		}
		out = append(out, CapPoint{Date: b.Date, Cap: b.Close * base[pickAt]})
	}
	return out
}

const (
	restatedWindow = 400 * 24 * time.Hour // сколько до сплита отчёт может быть показан в новой базе (прошлый год в сравнительных колонках)
	restatedAfter  = 120 * 24 * time.Hour // насколько после сплита искать первый отчёт в новой базе — опорный
)

// sharesToday — число акций каждого отчёта в сегодняшней базе: значение × сплиты после даты отчёта.
// Кадр SEC отдаёт значение из самого свежего отчёта, где период встречается, а в 10-K и 10-Q после сплита прошлый год показан уже в новой базе
// (у Apple баланс на 2019 год в кадре — 18 млрд вместо 4,5 млрд). Умножение на сплит давало бы число вдвое-вчетверо-вдесятеро больше настоящего (скачок на графике).
// Поэтому у отчёта за год до сплита сравниваются два варианта — «как есть» и «до сплита» — с первым отчётом после сплита (он заведомо в новой базе),
// и берётся тот, что ближе (и заметно ближе: акции за квартал-два так сильно не меняются).
func sharesToday(facts []Fact, splits []Split) []float64 {
	base := make([]float64, len(facts))
	for i, f := range facts {
		factor := 1.0
		for _, s := range splits {
			if s.Date > f.End {
				factor *= s.Ratio
			}
		}
		base[i] = f.Val * factor
	}
	for _, sp := range splits {
		if sp.Ratio <= 0 || sp.Ratio == 1 {
			continue
		}
		sd, err := time.Parse("2006-01-02", sp.Date)
		if err != nil {
			continue
		}
		ref := 0.0
		limit := sd.Add(restatedAfter).Format("2006-01-02")
		for i := range facts { // facts по возрастанию даты: первый отчёт, который закончился не раньше сплита
			if facts[i].End >= sp.Date && facts[i].End <= limit {
				ref = base[i]
				break
			}
		}
		if ref <= 0 {
			continue
		}
		from := sd.Add(-restatedWindow).Format("2006-01-02")
		for i := range facts {
			if facts[i].End >= sp.Date || facts[i].End < from {
				continue
			}
			as := math.Abs(math.Log(base[i] / ref))
			fixed := base[i] / sp.Ratio
			if math.Abs(math.Log(fixed/ref)) < 0.5*as {
				base[i] = fixed
			}
		}
	}
	return base
}

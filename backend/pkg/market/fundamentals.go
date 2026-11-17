package market

import (
	"sort"
	"strings"
)

// Отчётность F3 (probe9, 17.11.2026): выручка у компаний лежит под разными тегами — у Tesla есть и Revenues, и RevenueFromContract…, у Apple только второй,
// а до 2018 года у многих SalesRevenueNet, — поэтому за каждый год берётся первый тег цепочки, по которому у компании есть значение. Прибыль, капитал и активы
// — по одному тегу. Годовые кадры (CY2023) охватывают ~365 дней и у компаний с нестандартным финансовым годом приходятся на их ближайший год;
// капитал и активы — «на конец квартала», как и число акций.
const (
	TagRevenues         = "us-gaap/Revenues"
	TagRevenueContracts = "us-gaap/RevenueFromContractWithCustomerExcludingAssessedTax"
	TagRevenueSales     = "us-gaap/SalesRevenueNet"
	TagNetIncome        = "us-gaap/NetIncomeLoss"
	TagEquity           = "us-gaap/StockholdersEquity"
	TagAssets           = "us-gaap/Assets"
)

// RevenueChain — теги выручки по убыванию приоритета.
var RevenueChain = []string{TagRevenues, TagRevenueContracts, TagRevenueSales}

// FundPoint — значение на дату, $.
type FundPoint struct {
	Date string  `json:"d"`
	Val  float64 `json:"v"`
}

// Fundamentals — отчётность компании: выручка и чистая прибыль за год, капитал и активы на конец квартала; по возрастанию даты.
type Fundamentals struct {
	Revenue   []FundPoint `json:"revenue"`
	NetIncome []FundPoint `json:"net_income"`
	Equity    []FundPoint `json:"equity"`
	Assets    []FundPoint `json:"assets"`
}

// FundamentalTags — все теги, которые читает BuildFundamentals.
func FundamentalTags() []string {
	return append(append([]string{}, RevenueChain...), TagNetIncome, TagEquity, TagAssets)
}

// BuildFundamentals собирает отчётность из сохранённых значений одной компании. Выручка за период — по первому тегу цепочки, у которого она есть.
func BuildFundamentals(facts []Fact) Fundamentals {
	rank := map[string]int{}
	for i, t := range RevenueChain {
		rank[t] = i
	}
	revenue := map[string]Fact{}
	plain := map[string]map[string]Fact{TagNetIncome: {}, TagEquity: {}, TagAssets: {}}
	for _, f := range facts {
		if f.End == "" || f.Val == 0 {
			continue
		}
		if r, ok := rank[f.Tag]; ok {
			if old, have := revenue[f.Period]; !have || r < rank[old.Tag] {
				revenue[f.Period] = f
			}
			continue
		}
		if m, ok := plain[f.Tag]; ok {
			m[f.Period] = f
		}
	}
	// revenue и прибыль — только годовые периоды (CY2023), капитал и активы — только на конец квартала (CY2023Q4I)
	annual := func(m map[string]Fact) []FundPoint {
		return points(m, func(p string) bool { return !strings.HasSuffix(p, "I") })
	}
	instant := func(m map[string]Fact) []FundPoint {
		return points(m, func(p string) bool { return strings.HasSuffix(p, "I") })
	}
	return Fundamentals{
		Revenue:   annual(revenue),
		NetIncome: annual(plain[TagNetIncome]),
		Equity:    instant(plain[TagEquity]),
		Assets:    instant(plain[TagAssets]),
	}
}

func points(m map[string]Fact, keep func(period string) bool) []FundPoint {
	out := []FundPoint{}
	for p, f := range m {
		if keep(p) {
			out = append(out, FundPoint{Date: f.End, Val: f.Val})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

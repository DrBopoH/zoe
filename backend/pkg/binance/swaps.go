package binance

import (
	"fmt"
	"sort"
	"strings"
)

// Swap — обмен монет внутри кошелька: одна монета (или несколько) ушла, другая пришла. Капитал не меняет (оценка по рынку
// остаётся той же) — строка истории с бейджем вида, как покупки и переводы между кошельками.
//
//	convert — «Binance Convert»: одна монета в другую
//	dust    — «Small Assets Exchange BNB»: мелкие остатки (пыль) разных монет в BNB
//	stake   — «… Staking …»: монета → её токен стейкинга (ETH → WBETH, SOL → BNSOL) и обратно
type Swap struct {
	ID     string `json:"id"`
	T      int64  `json:"t"` // мс
	Wallet string `json:"wallet"`
	Kind   string `json:"kind"` // convert | dust | stake
	Op     string `json:"op"`
	Out    []Leg  `json:"out"` // что ушло
	In     []Leg  `json:"in"`  // что пришло
}

// Leg — одна нога обмена.
type Leg struct {
	Asset string  `json:"asset"`
	Qty   float64 `json:"qty"`
}

// SwapKind — вид обмена по названию операции журнала; "" — это не обмен.
func SwapKind(op string) string {
	switch {
	case op == "Binance Convert":
		return "convert"
	case strings.HasPrefix(op, "Small Assets Exchange"):
		return "dust"
	case strings.Contains(op, "Staking"): // «WBETH2.0 - Staking», «SOL Staking - Purchase»; названия обратных операций предположены
		return "stake"
	}
	return ""
}

type swapKey struct {
	t       int64
	account string
	op      string
}

// Swaps — обмены кошельков портфеля. Ноги одного обмена — строки одной секунды и одной операции: минус — ушло, плюс — пришло.
func Swaps(rows []Row) []Swap {
	by := map[swapKey]*Swap{}
	var order []swapKey
	for _, r := range rows {
		if r.Delta == 0 || !InPortfolio(r) || SwapKind(r.Op) == "" {
			continue
		}
		k := swapKey{r.T, r.Account, r.Op}
		sw := by[k]
		if sw == nil {
			sw = &Swap{T: r.T, Wallet: r.Account, Kind: SwapKind(r.Op), Op: r.Op}
			by[k] = sw
			order = append(order, k)
		}
		if r.Delta < 0 {
			sw.Out = append(sw.Out, Leg{r.Asset, -r.Delta})
		} else {
			sw.In = append(sw.In, Leg{r.Asset, r.Delta})
		}
	}
	var out []Swap
	seen := map[string]int{}
	for _, k := range order {
		sw := *by[k]
		if len(sw.Out) == 0 || len(sw.In) == 0 { // одна половина без пары — не обмен, остатки она и так двигает
			continue
		}
		base := fmt.Sprintf("sw|%d|%s|%s|%s|%.12g", k.t, k.account, k.op, sw.Out[0].Asset, sw.Out[0].Qty)
		seen[base]++
		sw.ID = fmt.Sprintf("%s|%d", base, seen[base])
		out = append(out, sw)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T > out[j].T })
	return out
}

package binance

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// WalletEarn — Simple Earn: в выписке своих строк у него нет, виден только по подписке и выкупу в Spot/Funding.
const WalletEarn = "Earn"

// Move — перемещение денег между кошельками внутри портфеля (Spot ↔ Funding / Strategy / USD-M Futures, Spot ↔ Earn).
// Капитал не меняет — это строка истории с бейджами «откуда → куда».
type Move struct {
	ID    string  `json:"id"`
	T     int64   `json:"t"` // мс
	From  string  `json:"from"`
	To    string  `json:"to"`
	Asset string  `json:"asset"`
	Qty   float64 `json:"qty"`
	// Auto — автоподписка: Binance сам кладёт начисленные проценты обратно в Earn, обычно в тот же день (та же монета и сумма).
	// Отдельным переводом это не событие: клиент сворачивает такую строку в само начисление.
	Auto bool `json:"auto,omitempty"`
}

// «Transfer Between Spot and UM Futures» → кошельки по именам из журнала
var walletAlias = map[string]string{"UM Futures": WalletFutures}

func walletName(s string) string {
	s = strings.TrimSpace(s)
	if w, ok := walletAlias[s]; ok {
		return w
	}
	return s
}

// otherSide — второй кошелёк перевода по названию операции; "" — не разобрать.
func otherSide(op, self string) string {
	rest := strings.TrimPrefix(op, "Transfer Between ")
	a, b, ok := strings.Cut(rest, " and ")
	if !ok {
		return ""
	}
	a, b = walletName(a), walletName(b)
	switch self {
	case a:
		return b
	case b:
		return a
	}
	return ""
}

// pairWindow — половины одного перевода в журнале могут отличаться на секунду (08:11:56 и 08:11:57).
const pairWindow = 3000

// Moves — переводы между кошельками и в Earn/из Earn. Перевод между кошельками в журнале двумя строками (минус в одном, плюс в другом):
// пары ищутся по монете и сумме в пределах pairWindow; одиночная половина достраивается по названию операции.
func Moves(rows []Row) []Move {
	var out []Move
	seen := map[string]int{}
	add := func(t int64, from, to, asset string, qty float64) {
		base := fmt.Sprintf("mv|%d|%s|%s|%s|%.12g", t, from, to, asset, qty)
		seen[base]++
		out = append(out, Move{ID: fmt.Sprintf("%s|%d", base, seen[base]), T: t, From: from, To: to, Asset: asset, Qty: qty})
	}
	var halves, accruals []Row
	for _, r := range rows {
		if !InPortfolio(r) || r.Delta == 0 {
			continue
		}
		if r.Delta > 0 && Kind(r) == KindDividend {
			accruals = append(accruals, r)
		}
		switch {
		case IsEarnMove(r):
			if r.Delta < 0 {
				add(r.T, r.Account, WalletEarn, r.Asset, -r.Delta)
			} else {
				add(r.T, WalletEarn, r.Account, r.Asset, r.Delta)
			}
		case IsInternalMove(r):
			halves = append(halves, r)
		}
	}
	sort.SliceStable(halves, func(i, j int) bool { return halves[i].T < halves[j].T })
	used := make([]bool, len(halves))
	for i, a := range halves {
		if used[i] || a.Delta > 0 {
			continue
		}
		for j, b := range halves {
			if used[j] || j == i || b.Delta < 0 || b.Asset != a.Asset || math.Abs(b.Delta+a.Delta) > 1e-8 || b.Account == a.Account {
				continue
			}
			if d := b.T - a.T; d < -pairWindow || d > pairWindow {
				continue
			}
			used[i], used[j] = true, true
			add(min(a.T, b.T), a.Account, b.Account, a.Asset, -a.Delta)
			break
		}
	}
	for i, r := range halves { // половина без пары: второй кошелёк — по названию операции
		if used[i] {
			continue
		}
		o := otherSide(r.Op, r.Account)
		switch {
		case o == "":
		case r.Delta < 0:
			add(r.T, r.Account, o, r.Asset, -r.Delta)
		default:
			add(r.T, o, r.Account, r.Asset, r.Delta)
		}
	}
	markAuto(out, accruals)
	sort.SliceStable(out, func(i, j int) bool { return out[i].T > out[j].T })
	return out
}

// autoWindow — насколько позже начисления Binance может само положить его в Earn (ночь через полночь укладывается).
const autoWindow = 36 * 60 * 60 * 1000

// markAuto помечает Spot/Funding → Earn, повторяющие начисление: та же монета и сумма, позже начисления не больше чем на autoWindow.
// Каждое начисление «гасит» не больше одной подписки.
func markAuto(moves []Move, accruals []Row) {
	sort.SliceStable(accruals, func(i, j int) bool { return accruals[i].T < accruals[j].T })
	idx := make([]int, 0, len(moves))
	for i, m := range moves {
		if m.To == WalletEarn {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool { return moves[idx[a]].T < moves[idx[b]].T })
	used := make([]bool, len(accruals))
	for _, i := range idx {
		m := moves[i]
		for j, a := range accruals {
			if used[j] || a.Asset != m.Asset || math.Abs(a.Delta-m.Qty) > 1e-8 {
				continue
			}
			if d := m.T - a.T; d < 0 || d > autoWindow {
				continue
			}
			used[j] = true
			moves[i].Auto = true
			break
		}
	}
}

package binance

import (
	"sort"
	"strings"
)

// Что строка выписки значит для портфеля. Правила явные и короткие; всё, чего в них нет, остаётся внутренним
// движением своего кошелька (сделки, комиссии, конвертации, пыль, стейкинг): оно меняет остатки, но не «приходом/уходом».

// Внешние движения — те, что становятся операциями счёта и показывают, как капитал входит и выходит.
const (
	KindDeposit  = "deposit"
	KindWithdraw = "withdraw"
	KindP2P      = "p2p"
	KindDividend = "dividend" // проценты и награды Earn
	KindBonus    = "bonus"    // кэшбэк и возврат комиссий
)

var extLabels = map[string]string{
	KindDeposit: "депозит", KindWithdraw: "вывод", KindP2P: "P2P", KindDividend: "начисление",
	KindBonus: "бонус",
}

// ExtLabel — подпись внешнего движения; ok == false у внутренних.
func ExtLabel(kind string) (string, bool) { l, ok := extLabels[kind]; return l, ok }

// ExtKinds — виды внешних движений, которые становятся операциями счёта.
func ExtKinds() []string {
	return []string{KindDeposit, KindWithdraw, KindP2P, KindDividend, KindBonus}
}

// Кошельки портфеля: все четыре. Деньги в ботах (Strategy) и на фьючерсах — те же деньги на Binance, их результат
// (сделки, комиссии, Realized PnL, Funding Fee) приходит строками этих кошельков, поэтому свечи двигаются по ходу работы.
const (
	WalletSpot     = "Spot"
	WalletFunding  = "Funding"
	WalletStrategy = "Strategy"
	WalletFutures  = "USD-M Futures"
)

// Kind — вид внешнего движения строки ("" — внутреннее).
func Kind(r Row) string {
	switch r.Op {
	case "Deposit":
		return KindDeposit
	case "Withdraw", "Withdrawal": // образца выписки с выводом пока нет: имя операции предположено
		return KindWithdraw
	case "P2P Trading":
		return KindP2P
	case "Cashback Voucher", "Strategy Trading Fee Rebate":
		return KindBonus
	}
	if strings.HasPrefix(r.Op, "Simple Earn") && (strings.HasSuffix(r.Op, "Interest") || strings.HasSuffix(r.Op, "Rewards")) {
		return KindDividend
	}
	return ""
}

// IsEarnMove — перевод между Spot/Funding и Simple Earn: деньги остаются на Binance, просто лежат в другом кошельке.
func IsEarnMove(r Row) bool {
	return strings.HasPrefix(r.Op, "Simple Earn") && (strings.HasSuffix(r.Op, "Subscription") || strings.HasSuffix(r.Op, "Redemption"))
}

// IsInternalMove — перевод между кошельками (Spot ↔ Funding / Strategy / Futures): обе стороны в журнале, в сумме ноль.
func IsInternalMove(r Row) bool { return strings.HasPrefix(r.Op, "Transfer Between ") }

// InPortfolio — строка меняет остатки портфеля (Spot + Funding + Strategy + Futures + Earn).
func InPortfolio(r Row) bool {
	switch r.Account {
	case WalletSpot, WalletFunding, WalletStrategy, WalletFutures:
		return true
	}
	return false
}

// Flows — изменения суммарных остатков монет. Переводы между кошельками и в Earn внутри портфеля, суммы не меняют.
func Flows(rows []Row) []Flow {
	var out []Flow
	for _, r := range rows {
		if !InPortfolio(r) || IsEarnMove(r) || IsInternalMove(r) || r.Delta == 0 {
			continue
		}
		out = append(out, Flow{T: r.T, Asset: r.Asset, Delta: r.Delta, Kind: Kind(r)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

// HoldingsOf — остатки по кошелькам на конец журнала: Spot, Funding, Strategy и Futures — сумма строк своего счёта,
// Earn — всё, что в Earn ушло (подписка уменьшает Spot/Funding, поэтому Earn = −Σ строк подписки/выкупа).
func HoldingsOf(rows []Row) Holdings {
	h := Holdings{}
	spot, fund, earn := map[string]float64{}, map[string]float64{}, map[string]float64{}
	strat, fut := map[string]float64{}, map[string]float64{}
	for _, r := range rows {
		if IsEarnMove(r) && InPortfolio(r) {
			earn[r.Asset] -= r.Delta
		}
		switch r.Account {
		case WalletSpot:
			spot[r.Asset] += r.Delta
		case WalletFunding:
			fund[r.Asset] += r.Delta
		case WalletStrategy:
			strat[r.Asset] += r.Delta
		case WalletFutures:
			fut[r.Asset] += r.Delta
		}
	}
	h.AddSpot(spot)
	h.AddFunding(fund)
	h.AddEarn(earn)
	h.AddStrategy(strat)
	h.AddFutures(fut)
	return h
}

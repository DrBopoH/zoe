package binance

import (
	"math"
	"strings"
	"time"
)

// Пояс времени у двух выписок одного счёта (журнал и история P2P) задаётся каждой своим именем файла «…(UTC+2)…».
// Если пометку убрали или она неверна, время ордеров и время зачислений расходится на целые часы.
// Лишний источник правды — сами данные: покупка по P2P появляется в Funding («P2P Trading») на ту же сумму через минуты после создания ордера.

const (
	zoneStep       = 15 * time.Minute // пояса кратны 15 минутам
	zoneMaxShift   = 14 * time.Hour   // от UTC−12 до UTC+14 между двумя файлами
	p2pMaxCreditAt = 60 * time.Minute // зачисление приходит не позже часа после создания ордера (обычно 3–10 минут)
	p2pCreditEps   = 0.005            // допуск суммы: в выписке 8 знаков, в истории ордеров сумма «количество − комиссия»
	zoneMinPairs   = 2                // меньше двух совпавших пар — вывод ненадёжен
)

// DetectShift — на сколько нужно сдвинуть время ордеров, чтобы их зачисления встали на место в журнале (кратно 15 минутам).
// 0 — сдвиг не нужен или данных мало: нужен минимум zoneMinPairs завершённых покупок с зачислением на ту же сумму,
// и сдвиг принимается, только если с ним совпадает строго больше пар, чем без него. Исходное время имеет приоритет: при равенстве — 0.
// pairs — сколько ордеров встало на место при найденном сдвиге.
func DetectShift(orders []P2POrder, ledger []Row) (shift time.Duration, pairs int) {
	// разницы «время зачисления − время ордера» по всем подходящим по сумме зачислениям каждого ордера
	var diffs [][]time.Duration
	for _, o := range orders {
		if !o.Done() || !strings.EqualFold(o.Side, "buy") {
			continue
		}
		var ds []time.Duration
		for _, r := range ledger {
			if r.Op == "P2P Trading" && r.Asset == o.Asset && r.Delta > 0 && math.Abs(r.Delta-o.Credit()) < p2pCreditEps {
				ds = append(ds, time.Duration(r.T-o.T)*time.Millisecond)
			}
		}
		if len(ds) > 0 {
			diffs = append(diffs, ds)
		}
	}
	fits := func(s time.Duration) int {
		n := 0
		for _, ds := range diffs {
			for _, d := range ds {
				if d-s >= 0 && d-s <= p2pMaxCreditAt {
					n++
					break
				}
			}
		}
		return n
	}
	best, bestN := time.Duration(0), fits(0)
	for _, ds := range diffs {
		for _, d := range ds {
			s := (d - 5*time.Minute).Round(zoneStep) // типичная задержка зачисления — минуты
			if s == 0 || s > zoneMaxShift || s < -zoneMaxShift {
				continue
			}
			if n := fits(s); n > bestN || (n == bestN && best != 0 && abs(s) < abs(best)) {
				best, bestN = s, n
			}
		}
	}
	if best == 0 || bestN < zoneMinPairs {
		return 0, 0
	}
	return best, bestN
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

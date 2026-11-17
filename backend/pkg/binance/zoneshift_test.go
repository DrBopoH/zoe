package binance

import (
	"testing"
	"time"
)

func buyOrder(id string, at time.Time, credit float64) P2POrder {
	return P2POrder{ID: id, Side: "Buy", Asset: "USDT", Status: "Completed", Qty: credit + 0.05, Fee: 0.05, T: at.UnixMilli()}
}

func p2pCredit(at time.Time, v float64) Row {
	return Row{T: at.UnixMilli(), Account: "Funding", Op: "P2P Trading", Asset: "USDT", Delta: v}
}

// Ордера прочитаны в поясе на 2 часа «раньше», чем журнал: зачисления кажутся пришедшими через 2 ч 5 мин.
func TestDetectShiftFindsWholeHours(t *testing.T) {
	base := time.Date(2026, 5, 19, 7, 56, 7, 0, time.UTC)
	var orders []P2POrder
	var ledger []Row
	for i, v := range []float64{25.06, 88.85, 40.1, 12.34} {
		at := base.Add(time.Duration(i) * 26 * time.Hour)
		ledger = append(ledger, p2pCredit(at.Add(6*time.Minute), v))
		orders = append(orders, buyOrder(string(rune('a'+i)), at.Add(-2*time.Hour), v)) // ордер «на 2 часа раньше»
	}
	shift, pairs := DetectShift(orders, ledger)
	if shift != 2*time.Hour || pairs != 4 {
		t.Fatalf("сдвиг %v, пар %d", shift, pairs)
	}
	// после сдвига сдвиг больше не нужен
	for i := range orders {
		orders[i].T += shift.Milliseconds()
	}
	if shift, _ := DetectShift(orders, ledger); shift != 0 {
		t.Fatalf("после выравнивания сдвиг должен быть 0, а %v", shift)
	}
}

func TestDetectShiftNegativeAndHalfHourZones(t *testing.T) {
	base := time.Date(2026, 5, 19, 7, 56, 7, 0, time.UTC)
	for _, want := range []time.Duration{-3 * time.Hour, 5*time.Hour + 30*time.Minute} {
		var orders []P2POrder
		var ledger []Row
		for i, v := range []float64{25.06, 88.85, 40.1} {
			at := base.Add(time.Duration(i) * 30 * time.Hour)
			ledger = append(ledger, p2pCredit(at.Add(4*time.Minute), v))
			orders = append(orders, buyOrder(string(rune('a'+i)), at.Add(-want), v))
		}
		if shift, pairs := DetectShift(orders, ledger); shift != want || pairs != 3 {
			t.Fatalf("ждали %v, получили %v (%d пар)", want, shift, pairs)
		}
	}
}

// Время совпадает — сдвига нет; мало пар, отменённые ордера и продажи не считаются.
func TestDetectShiftNeedsEvidence(t *testing.T) {
	at := time.Date(2026, 5, 19, 7, 56, 7, 0, time.UTC)
	ok := buyOrder("a", at, 25.06)
	led := []Row{p2pCredit(at.Add(6*time.Minute), 25.06)}
	if s, _ := DetectShift([]P2POrder{ok}, led); s != 0 {
		t.Fatalf("время сходится: %v", s)
	}
	// одна пара с расхождением — мало данных, сдвиг не выводится
	if s, _ := DetectShift([]P2POrder{buyOrder("a", at.Add(-2*time.Hour), 25.06)}, led); s != 0 {
		t.Fatalf("одной пары мало: %v", s)
	}
	// две пары, но отменённые и продажи не считаются
	c1, c2 := buyOrder("b", at.Add(-2*time.Hour), 88.85), buyOrder("c", at.Add(-2*time.Hour), 40.1)
	c1.Status = "Cancelled"
	c2.Side = "Sell"
	led2 := []Row{p2pCredit(at.Add(6*time.Minute), 88.85), p2pCredit(at.Add(6*time.Minute), 40.1)}
	if s, _ := DetectShift([]P2POrder{c1, c2}, led2); s != 0 {
		t.Fatalf("отменённые и продажи не считаются: %v", s)
	}
	// ничьей нет: если с нулевым сдвигом совпадает столько же, сколько с любым другим, остаётся 0
	if s, _ := DetectShift(nil, nil); s != 0 {
		t.Fatal("пустые данные")
	}
}

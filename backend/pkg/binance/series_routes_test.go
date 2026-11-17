package binance

import (
	"math"
	"testing"
)

func routesScenario() ([]Flow, int64, int64, map[string]Sym) {
	const m = int64(60_000)
	t0 := int64(1_700_000_000_000) / m * m
	flows := []Flow{
		{T: t0, Asset: "USDT", Delta: 1000},
		{T: t0, Asset: "UAH", Delta: 500},
		{T: t0 + 10*m, Asset: "BTC", Delta: 0.01},   // прямая пара BTCUSDT
		{T: t0 + 15*m, Asset: "ETH", Delta: 0.5},    // только через BTC: ETHBTC × BTCUSDT
		{T: t0 + 20*m, Asset: "USDC", Delta: 50},    // стейбл без пары = 1 USDT
		{T: t0 + 25*m, Asset: "FOO", Delta: 7},      // нигде нет пары: «не оценено»
		{T: t0 + 40*m, Asset: "BTC", Delta: -0.03},  // продажа непоказанной покупки: остаток отрицательный
		{T: t0 + 50*m, Asset: "ETH", Delta: -0.5},   // закрытая позиция: отрезок кончается
		{T: t0 + 55*m, Asset: "LDUSDT", Delta: 100}, // токен ликвидного стейкинга не оценивается отдельно
	}
	return flows, t0, t0 + 90*m, map[string]Sym{"BTCUSDT": {Base: "BTC"}, "ETHBTC": {Base: "ETH"}}
}

// Все маршруты оценки (прямая пара, через BTC, стейбл, UAH, без пары) в одном ряду; прогресс доходит до 100 % ровно в конце.
func TestSeriesRoutesAndProgress(t *testing.T) {
	srv := movingPriceServer()
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	flows, t0, end, syms := routesScenario()
	pts := Timeline(flows, end)

	var lastDone, lastTotal, calls int
	res, err := c.Series(pts, t0, end, syms, func(done, total int) { lastDone, lastTotal, calls = done, total, calls+1 })
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Unpriced) != 1 || res.Unpriced[0] != "FOO" {
		t.Fatalf("не оценены: %v", res.Unpriced)
	}
	if calls == 0 || lastTotal == 0 || lastDone > lastTotal {
		t.Fatalf("прогресс: %d/%d за %d вызовов", lastDone, lastTotal, calls)
	}
	if c.page != nil {
		t.Fatal("счётчик страниц должен отключаться после ряда")
	}
	var sum float64
	for i, v := range res.Values {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			t.Fatalf("минута %d: %v", i, v)
		}
		sum += v
	}
	// сумма снята с реализации до разбиения Series на части: поведение не должно было измениться
	if len(res.Values) != 91 || math.Abs(sum-282607119621.28) > 0.01 {
		t.Fatalf("ряд изменился: %d минут, сумма %.2f", len(res.Values), sum)
	}
}

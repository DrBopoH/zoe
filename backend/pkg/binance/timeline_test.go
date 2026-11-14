package binance

import (
	"math"
	"math/rand"
	"testing"
)

// Покупки и продажи на больших количествах: закрытая позиция обязана стать ровно нулём, а не 1e-7-хвостом.
func TestTimelineClosesPositionAtLargeScale(t *testing.T) {
	for _, scale := range []float64{1, 1e3, 1e6, 1e9} {
		rng := rand.New(rand.NewSource(7))
		var fl []Flow
		var held []float64
		for i := 0; i < 200; i++ {
			q := math.Round(rng.Float64()*scale*1e8) / 1e8
			held = append(held, q)
			fl = append(fl, Flow{T: int64(2*i + 1), Asset: "X", Delta: q})
		}
		for i, q := range held {
			fl = append(fl, Flow{T: int64(1000 + i), Asset: "X", Delta: -q})
		}
		pts := Timeline(fl, 5000)
		last := pts[len(pts)-1]
		if last.H["X"] != 0 {
			t.Fatalf("scale %g: остаток %g вместо 0", scale, last.H["X"])
		}
	}
}

func TestTimelineBasics(t *testing.T) {
	pts := Timeline([]Flow{
		{T: 20, Asset: "A", Delta: -0.5},
		{T: 10, Asset: "A", Delta: 1.25},
		{T: 10, Asset: "B", Delta: 3},
	}, 15)
	if len(pts) != 3 || pts[0].T != 10 || pts[1].T != 20 || pts[2].T != 21 {
		t.Fatalf("точки: %+v", pts)
	}
	if pts[0].H["A"] != 1.25 || pts[0].H["B"] != 3 || pts[2].H["A"] != 0.75 {
		t.Fatalf("остатки: %+v", pts)
	}
	if Timeline(nil, 5) != nil {
		t.Fatal("пустой вход — пустой результат")
	}
}

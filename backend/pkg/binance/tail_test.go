package binance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// Цена зависит от минуты и пары: так видно, что хвост считается по тем же ценам, что и полный ряд.
func movingPriceServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		price := func(sym string, ts int64) float64 {
			if sym == "USDTUAH" {
				return 40 + float64(ts/60000%7)/10
			}
			return 20000 + float64(ts/60000%50)
		}
		sym := q.Get("symbol")
		row := func(ts int64) string {
			return fmt.Sprintf(`[%d,"0","0","0","%g","0",0,"0",0,"0","0","0"]`, ts, price(sym, ts))
		}
		var rows []string
		if q.Get("startTime") == "" {
			end, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
			rows = append(rows, row(end/60000*60000))
		} else {
			from, _ := strconv.ParseInt(q.Get("startTime"), 10, 64)
			to, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
			for ts := (from + 59999) / 60000 * 60000; ts <= to && len(rows) < 1000; ts += 60000 {
				rows = append(rows, row(ts))
			}
		}
		fmt.Fprint(w, "["+strings.Join(rows, ",")+"]")
	}))
}

// Хвост, посчитанный от середины, совпадает с тем же хвостом полного ряда — в том числе когда внутри хвоста есть движения:
// именно так дорисовывается ряд после новой выписки, не собираясь заново с начала.
func TestSeriesTailMatchesFullSeries(t *testing.T) {
	srv := movingPriceServer()
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	syms := map[string]Sym{"BTCUSDT": {Base: "BTC"}}

	const m = int64(60_000)
	t0 := int64(1_700_000_000_000) / m * m
	flows := []Flow{
		{T: t0, Asset: "USDT", Delta: 1000},
		{T: t0 + 20*m, Asset: "BTC", Delta: 0.01},
		{T: t0 + 20*m, Asset: "USDT", Delta: -200},
		{T: t0 + 70*m, Asset: "BTC", Delta: 0.02}, // движение внутри хвоста
		{T: t0 + 70*m, Asset: "USDT", Delta: -400},
	}
	end := t0 + 120*m
	pts := Timeline(flows, end)
	full, err := c.Series(pts, t0, end, syms, nil)
	if err != nil {
		t.Fatal(err)
	}
	from := t0 + 50*m // обрезанный ряд кончился здесь
	tail, err := c.Series(pts, from, end, syms, nil)
	if err != nil {
		t.Fatal(err)
	}
	off := int((from - t0) / m)
	if len(tail.Values) != len(full.Values)-off {
		t.Fatalf("длина хвоста %d, ждали %d", len(tail.Values), len(full.Values)-off)
	}
	for i, v := range tail.Values {
		if v != full.Values[off+i] {
			t.Fatalf("минута +%d: хвост %v, полный ряд %v", off+i, v, full.Values[off+i])
		}
	}
}

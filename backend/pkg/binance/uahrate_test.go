package binance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// Binance, у которого пара USDTUAH снята с торгов в минуту cut: свечей пары с этой минуты нет, остальные пары живы.
// Запросы отвечают как настоящий: endTime+limit=1 — последняя свеча не позже, startTime+limit=1 — первая не раньше.
func delistedServer(cut int64) *httptest.Server {
	const m = int64(60_000)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sym := q.Get("symbol")
		has := func(ts int64) bool { return sym != rateSym || ts < cut }
		price := "20000"
		if sym == rateSym {
			price = "42"
		}
		row := func(ts int64) string { return fmt.Sprintf(`[%d,"0","0","0","%s","0",0,"0",0,"0","0","0"]`, ts, price) }
		from, hasFrom := int64(0), q.Get("startTime") != ""
		to, hasTo := int64(1)<<60, q.Get("endTime") != ""
		if hasFrom {
			from, _ = strconv.ParseInt(q.Get("startTime"), 10, 64)
		}
		if hasTo {
			to, _ = strconv.ParseInt(q.Get("endTime"), 10, 64)
		}
		limit, _ := strconv.Atoi(q.Get("limit"))
		var rows []string
		switch {
		case hasTo && !hasFrom: // последняя свеча не позже to
			ts := to / m * m
			if sym == rateSym && ts >= cut {
				ts = cut - m
			}
			rows = append(rows, row(ts))
		default:
			for ts := (from + m - 1) / m * m; ts <= to && len(rows) < limit; ts += m {
				if !has(ts) {
					break
				}
				rows = append(rows, row(ts))
			}
		}
		fmt.Fprint(w, "["+strings.Join(rows, ",")+"]")
	}))
}

// Ряд через момент снятия пары: до него курс из свечей, после — внешний (НБУ), а без внешнего — последняя цена пары.
func TestSeriesAfterPairDelisted(t *testing.T) {
	const m = int64(60_000)
	t0 := int64(1_700_000_000_000) / m * m
	cut := t0 + 30*m
	srv := delistedServer(cut)
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	flows := []Flow{{T: t0, Asset: "USDT", Delta: 100}}
	end := t0 + 60*m
	pts := Timeline(flows, end)

	c.UAHRate = func(tMs int64) float64 { return 41 }
	res, err := c.Series(pts, t0, end, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Values) != 61 || res.Values[0] != 4200 || res.Values[29] != 4200 || res.Values[30] != 4100 || res.Values[60] != 4100 {
		t.Fatalf("стык курсов: %v", res.Values)
	}

	// хвост целиком после снятия пары: в окне нет ни одной свечи — раньше здесь был отказ «нет минутных цен»
	tail, err := c.Series(pts, cut+5*m, end, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range tail.Values {
		if v != 4100 || v != res.Values[35+i] {
			t.Fatalf("хвост расходится с полным рядом на минуте %d: %v", i, v)
		}
	}

	c.UAHRate = nil // внешнего курса нет: держим последнюю цену пары, а не падаем
	tail, err = c.Series(pts, cut+5*m, end, nil, nil)
	if err != nil || tail.Values[0] != 4200 || tail.Values[len(tail.Values)-1] != 4200 {
		t.Fatalf("без внешнего курса: %v %v", tail, err)
	}
}

func TestRateUAH(t *testing.T) {
	const m = int64(60_000)
	t0 := int64(1_700_000_000_000) / m * m
	cut := t0 + 30*m
	srv := delistedServer(cut)
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	c.UAHRate = func(tMs int64) float64 { return 41 }

	if r, ext, err := c.RateUAH(t0 + 10*m + 5); err != nil || ext || r != 42 {
		t.Fatalf("пока пара жива — её свеча: %v %v %v", r, ext, err)
	}
	if r, ext, err := c.RateUAH(cut + 100*m); err != nil || !ext || r != 41 {
		t.Fatalf("после снятия пары — внешний курс: %v %v %v", r, ext, err)
	}
	c.UAHRate = nil
	if r, ext, err := c.RateUAH(cut + 100*m); err != nil || ext || r != 42 {
		t.Fatalf("внешнего курса нет — последняя цена пары: %v %v %v", r, ext, err)
	}
	// движение после снятия пары оценивается по тому же курсу
	c.UAHRate = func(tMs int64) float64 { return 41 }
	if v, err := c.ValueUAH("USDT", 10, cut+100*m); err != nil || v != 410 {
		t.Fatalf("оценка движения: %v %v", v, err)
	}
}

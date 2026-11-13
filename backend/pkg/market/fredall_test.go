package market

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Пачка рядов качается параллельно, но не больше fredParallel сразу; результат и ошибки — в порядке заданий, прогресс растёт.
func TestFredAllParallelLimitedAndOrdered(t *testing.T) {
	var cur, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		cur.Add(-1)
		id := r.URL.Query().Get("id")
		if id == "BAD1" || id == "BAD2" {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		fmt.Fprintf(w, "observation_date,%s\n2026-01-01,1\n2026-02-01,.\n2026-03-01,3\n", id)
	}))
	defer srv.Close()
	c := NewClient()
	c.FREDBase = srv.URL

	jobs := []fredJob{{"A", "a"}, {"BAD1", "b1"}, {"C", "c"}, {"D", "d"}, {"BAD2", "b2"}, {"E", "e"}, {"F", "f"}, {"G", "g"}}
	var mu sync.Mutex
	var pcts []int
	r := c.fredAll(context.Background(), func(p int, _ string) { mu.Lock(); pcts = append(pcts, p); mu.Unlock() }, jobs)

	if got := peak.Load(); got < 2 || got > fredParallel {
		t.Fatalf("одновременных запросов %d, ожидалось 2…%d", got, fredParallel)
	}
	if len(r.Obs) != 6 || len(r.Obs["A"]) != 2 || r.Obs["BAD1"] != nil {
		t.Fatalf("ряды: %+v", r.Obs)
	}
	if r.Skipped != 6 { // по одной строке с пропуском «.» в шести удачных рядах
		t.Fatalf("пропущено %d", r.Skipped)
	}
	if len(r.Errs) != 2 || r.Errs[0][:9] != "FRED BAD1" || r.Errs[1][:9] != "FRED BAD2" {
		t.Fatalf("ошибки не в порядке заданий: %v", r.Errs)
	}
	if len(pcts) != len(jobs) {
		t.Fatalf("прогресс: %v", pcts)
	}
	for i := 1; i < len(pcts); i++ {
		if pcts[i] < pcts[i-1] {
			t.Fatalf("прогресс не должен идти назад: %v", pcts)
		}
	}
	if last := pcts[len(pcts)-1]; last != 90 {
		t.Fatalf("после последнего ряда 90 %%, получили %d", last)
	}
}

func TestFredAllNilReporterAndCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, "observation_date,X\n2026-01-01,1\n")
	}))
	defer srv.Close()
	c := NewClient()
	c.FREDBase = srv.URL
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	r := c.fredAll(ctx, nil, []fredJob{{"A", ""}, {"B", ""}})
	if len(r.Obs) != 0 || len(r.Errs) != 2 {
		t.Fatalf("отмена контекста: ошибка у каждого ряда, без зависания: %+v", r)
	}
}

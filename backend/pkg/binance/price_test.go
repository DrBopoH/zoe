package binance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// Пара без сделок после deadAfter: свечей позже нет (как неликвидная USDT/UAH). Отдаёт цену 40.
func deadPairServer(deadAfter int64, calls *atomic.Int64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if r.URL.Path != "/api/v3/klines" {
			w.WriteHeader(404)
			return
		}
		row := func(ts int64) string { return fmt.Sprintf(`[%d,"0","0","0","40","0",0,"0",0,"0","0","0"]`, ts) }
		var rows []string
		if q.Get("startTime") == "" { // только endTime+limit=1: последняя свеча не позже endTime
			end, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
			if end > deadAfter {
				end = deadAfter
			}
			rows = append(rows, row(end/60000*60000))
		} else {
			from, _ := strconv.ParseInt(q.Get("startTime"), 10, 64)
			to, _ := strconv.ParseInt(q.Get("endTime"), 10, 64)
			if to > deadAfter {
				to = deadAfter
			}
			for ts := (from + 59999) / 60000 * 60000; ts <= to && len(rows) < 1000; ts += 60000 {
				rows = append(rows, row(ts))
			}
		}
		fmt.Fprint(w, "["+strings.Join(rows, ",")+"]")
	}))
}

// Начисление после последней сделки пары оценивается по цене последней сделки, а не «нет цены».
func TestMinuteCloseOnDeadPairUsesLastTrade(t *testing.T) {
	var calls atomic.Int64
	srv := deadPairServer(1_000_000*60_000, &calls)
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	got, err := c.minuteClose("USDTUAH", 1_000_500*60_000)
	if err != nil || got != 40 {
		t.Fatalf("цена по последней сделке: %v %v", got, err)
	}
}

// Окна свечей качаются параллельно, ни одна минута не теряется и не дублируется; прогресс идёт по страницам.
func TestKlinesParallelWindowsAreComplete(t *testing.T) {
	var calls atomic.Int64
	srv := deadPairServer(1<<60, &calls)
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	c.Workers = 4
	from, n := int64(60_000), 3500 // 3,5 окна
	var mu sync.Mutex
	seen := map[int64]int{}
	var pageCalls atomic.Int64
	c.page = func() { pageCalls.Add(1) }
	if err := c.klines("USDTUAH", from, from+int64(n)*60_000-1, func(ts int64, p float64) {
		mu.Lock()
		seen[ts]++
		mu.Unlock()
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != n {
		t.Fatalf("минут %d, ждали %d", len(seen), n)
	}
	for ts, k := range seen {
		if k != 1 || ts < from || ts >= from+int64(n)*60_000 {
			t.Fatalf("минута %d: %d раз", ts, k)
		}
	}
	if pageCalls.Load() != int64(pages(n)) || calls.Load() != int64(pages(n)) {
		t.Fatalf("страниц %d, запросов %d, ждали %d", pageCalls.Load(), calls.Load(), pages(n))
	}
}

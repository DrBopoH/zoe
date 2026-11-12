package binance

import (
	"encoding/json"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Остатки на любой момент известны из журнала (ledger.go), поэтому история портфеля — это оценка этих остатков
// по минутным свечам монет и пары USDT/UAH. Цены публичные: ключ не нужен.

// Flow — изменение суммарного остатка монеты (по всем кошелькам) в момент T (мс).
type Flow struct {
	T     int64
	Asset string
	Delta float64
	Kind  string
}

// Point — остатки, действующие с момента T (мс) и до следующей точки.
type Point struct {
	T int64
	H map[string]float64
}

// Timeline — остатки после каждого движения (нарастающим итогом) и финальная точка endMs с теми же остатками.
func Timeline(flows []Flow, endMs int64) []Point {
	fl := append([]Flow(nil), flows...)
	sort.SliceStable(fl, func(i, j int) bool { return fl[i].T < fl[j].T })
	var pts []Point
	cur := map[string]float64{}
	snap := func() map[string]float64 {
		m := make(map[string]float64, len(cur))
		for a, v := range cur {
			m[a] = v
		}
		return m
	}
	for i := 0; i < len(fl); {
		t := fl[i].T
		for i < len(fl) && fl[i].T == t {
			a := fl[i].Asset
			cur[a] += fl[i].Delta
			if math.Abs(cur[a]) < dust {
				cur[a] = 0
			}
			i++
		}
		pts = append(pts, Point{T: t, H: snap()})
	}
	if n := len(pts); n > 0 {
		if pts[n-1].T >= endMs {
			endMs = pts[n-1].T + 1
		}
		pts = append(pts, Point{T: endMs, H: snap()})
	}
	return pts
}

func skipLD(a string) bool { return strings.HasPrefix(a, "LD") && len(a) > 2 }

const minute = int64(60_000)

// window — одна страница свечей: 1000 минут. Окна не зависят друг от друга, поэтому качаются параллельно.
const window = 1000 * minute

// pages — сколько запросов нужно на диапазон длиной n минут.
func pages(n int) int { return (n + 999) / 1000 }

// klines тянет 1m-свечи пары на [fromMs,toMs] и отдаёт (время открытия, закрытие). Окна по 1000 минут качаются
// параллельно (Workers); put вызывается из разных горутин, но для разных свечей.
func (c *Client) klines(symbol string, fromMs, toMs int64, put func(t int64, closeP float64)) error {
	var starts []int64
	for s := fromMs; s <= toMs; s += window {
		starts = append(starts, s)
	}
	workers := c.Workers
	if workers < 1 {
		workers = 4
	}
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		next    int
		failure error
	)
	one := func(ws int64) error {
		we := ws + window - 1
		if we > toMs {
			we = toMs
		}
		body, err := c.do("/api/v3/klines", url.Values{"symbol": {symbol}, "interval": {"1m"}, "startTime": {strconv.FormatInt(ws, 10)}, "endTime": {strconv.FormatInt(we, 10)}, "limit": {"1000"}})
		if err != nil {
			return err
		}
		var r [][]json.RawMessage
		if err := json.Unmarshal(body, &r); err != nil {
			return err
		}
		for _, k := range r {
			if len(k) < 5 {
				continue
			}
			var t float64
			var cl string
			_ = json.Unmarshal(k[0], &t)
			_ = json.Unmarshal(k[4], &cl)
			put(int64(t), num(cl))
		}
		if c.page != nil {
			c.page()
		}
		return nil
	}
	for w := 0; w < workers && w < len(starts); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if failure != nil || next >= len(starts) {
					mu.Unlock()
					return
				}
				ws := starts[next]
				next++
				mu.Unlock()
				if err := one(ws); err != nil {
					mu.Lock()
					if failure == nil {
						failure = err
					}
					mu.Unlock()
					return
				}
			}
		}()
	}
	wg.Wait()
	return failure
}

// priceGrid — цена пары на минутной сетке [g0, g0+n*minute): пропуски заполняются прошлой ценой, начало — первой известной.
func (c *Client) priceGrid(symbol string, g0 int64, n int, fromIdx, toIdx int) ([]float64, bool, error) {
	px := make([]float64, n)
	if fromIdx < 0 {
		fromIdx = 0
	}
	if toIdx > n-1 {
		toIdx = n - 1
	}
	if fromIdx > toIdx {
		return px, false, nil
	}
	err := c.klines(symbol, g0+int64(fromIdx)*minute, g0+int64(toIdx+1)*minute-1, func(t int64, p float64) {
		if i := int((t - g0) / minute); i >= 0 && i < n && p > 0 {
			px[i] = p
		}
	})
	if err != nil {
		return nil, false, err
	}
	first := -1
	for i := fromIdx; i <= toIdx; i++ {
		if px[i] > 0 {
			first = i
			break
		}
	}
	if first < 0 {
		return px, false, nil
	}
	for i := fromIdx; i < first; i++ {
		px[i] = px[first]
	}
	for i := first + 1; i <= toIdx; i++ {
		if px[i] == 0 {
			px[i] = px[i-1]
		}
	}
	return px, true, nil
}

// SeriesResult — минутный ряд оценки портфеля в ₴: Values[i] — на минуту StartMs + i*60000.
type SeriesResult struct {
	StartMs  int64
	Values   []float64
	Unpriced []string
}

// Series оценивает остатки pts на минутной сетке от g0 (мс, кратно минуте) до endMs.
// Монета → USDT → ₴: прямая пара XXXUSDT, иначе через BTC/ETH/BNB; стейблы без пары = 1 USDT; UAH — как есть.
func (c *Client) Series(pts []Point, g0, endMs int64, syms map[string]Sym, progress func(done, total int)) (*SeriesResult, error) {
	n := int((endMs-g0)/minute) + 1
	if n <= 0 || len(pts) == 0 {
		return &SeriesResult{StartMs: g0}, nil
	}
	idx := func(t int64) int {
		i := int((t - g0) / minute)
		if t < g0 {
			i = 0
		}
		return i
	}

	// когда монета лежит на счёте: [first, last] в индексах сетки
	type span struct{ a, b int }
	spans := map[string]span{}
	assets := map[string]bool{}
	for _, p := range pts {
		for a := range p.H {
			assets[a] = true
		}
	}
	for a := range assets {
		if skipLD(a) {
			continue
		}
		first, lastPos := -1, -1
		for k, p := range pts {
			if p.H[a] != 0 { // и отрицательные: продажа монеты, покупку которой API не показал (Alpha, стейкинг, раздача)
				if first < 0 {
					first = k
				}
				lastPos = k
			}
		}
		if first < 0 {
			continue
		}
		b := n - 1
		if lastPos+1 < len(pts) {
			b = idx(pts[lastPos+1].T)
		}
		spans[a] = span{idx(pts[first].T), b}
	}

	names := make([]string, 0, len(spans))
	for a := range spans {
		names = append(names, a)
	}
	sort.Strings(names)

	// прогресс — по запросам: заранее считаем, сколько страниц свечей нужно (курс, монеты, промежуточные BTC/ETH/BNB)
	if progress != nil {
		total, midSeen := pages(n), map[string]bool{}
		for _, a := range names {
			sp := spans[a]
			switch {
			case a == "UAH", a == "USDT":
			case syms[a+"USDT"].Base != "":
				total += pages(sp.b - sp.a + 1)
			case stable[a]:
			default:
				for _, m := range []string{"BTC", "ETH", "BNB"} {
					if syms[a+m].Base == "" {
						continue
					}
					total += pages(sp.b - sp.a + 1)
					if !midSeen[m] {
						midSeen[m] = true
						total += pages(n)
					}
					break
				}
			}
		}
		var (
			pm   sync.Mutex
			done int
		)
		c.page = func() {
			pm.Lock()
			defer pm.Unlock()
			if done < total-1 { // 100% — только когда ряд собран целиком
				done++
			}
			progress(done, total)
		}
		defer func() { c.page = nil }()
		progress(0, total)
	}

	rate, ok, err := c.priceGrid("USDTUAH", g0, n, 0, n-1)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errNoRate
	}

	usdt := make([]float64, n)
	uah := make([]float64, n)
	mids := map[string][]float64{}
	mid := func(sym string) ([]float64, bool, error) {
		if v, ok := mids[sym]; ok {
			return v, v != nil, nil
		}
		px, ok, err := c.priceGrid(sym, g0, n, 0, n-1)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			px = nil
		}
		mids[sym] = px
		return px, ok, nil
	}

	res := &SeriesResult{StartMs: g0}
	for _, a := range names {
		sp := spans[a]
		var px []float64
		switch {
		case a == "UAH":
			// 1 ₴ = 1 ₴
		case a == "USDT":
			px = ones(n)
		case syms[a+"USDT"].Base != "":
			p, ok, err := c.priceGrid(a+"USDT", g0, n, sp.a, sp.b)
			if err != nil {
				return nil, err
			}
			if ok {
				px = p
			} else if stable[a] {
				px = ones(n)
			}
		case stable[a]:
			px = ones(n)
		default:
			for _, m := range []string{"BTC", "ETH", "BNB"} {
				if syms[a+m].Base == "" {
					continue
				}
				p, ok, err := c.priceGrid(a+m, g0, n, sp.a, sp.b)
				if err != nil {
					return nil, err
				}
				mp, ok2, err := mid(m + "USDT")
				if err != nil {
					return nil, err
				}
				if ok && ok2 {
					px = make([]float64, n)
					for i := sp.a; i <= sp.b && i < n; i++ {
						px[i] = p[i] * mp[i]
					}
					break
				}
			}
		}
		if px == nil && a != "UAH" {
			res.Unpriced = append(res.Unpriced, a)
			continue
		}
		// остатки меняются в точках pts; идём по ним параллельно с сеткой
		pk := 0
		for pk+1 < len(pts) && idx(pts[pk+1].T) <= sp.a {
			pk++
		}
		for i := sp.a; i <= sp.b && i < n; i++ {
			for pk+1 < len(pts) && idx(pts[pk+1].T) <= i {
				pk++
			}
			// Отрицательный остаток — «дыра» от непоказанной покупки: продажа добавила USDT, а монеты в учёте не было.
			// Считаем её по рынку, чтобы она гасила эти USDT, а не оставляла на графике завышенный хвост, который потом рухнет.
			h := pts[pk].H[a]
			if pts[pk].T > g0+int64(i)*minute+minute-1 || h == 0 {
				continue
			}
			if a == "UAH" {
				uah[i] += h
			} else {
				usdt[i] += h * px[i]
			}
		}
	}
	res.Values = make([]float64, n)
	for i := range res.Values {
		res.Values[i] = round2(usdt[i]*rate[i] + uah[i])
	}
	return res, nil
}

func ones(n int) []float64 {
	o := make([]float64, n)
	for i := range o {
		o[i] = 1
	}
	return o
}

type histErr string

func (e histErr) Error() string { return string(e) }

const errNoRate = histErr("нет минутных цен USDT/UAH — оценить историю в гривнах нечем")

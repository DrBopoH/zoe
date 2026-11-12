package binance

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// История портфеля задним числом. Binance не отдаёт «баланс на момент T», поэтому прошлое восстанавливается:
// берём остатки сейчас и откатываем назад все известные движения (депозиты, выводы, сделки, конвертации, P2P,
// покупки картой, дивиденды), затем оцениваем остатки по минутным свечам монет и пары USDT/UAH.
// Это приблизительно: что API не отдаёт (проценты Earn, пыль, Pay и т.п.) остаётся «необъяснённым» и показывается отдельно.

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

// Timeline откатывает движения назад от остатков now и возвращает точки «остатки после каждого движения»
// плюс финальную точку endMs с остатками now. Необъяснённое (now − Σдвижений) в историю не входит и появляется
// одним скачком в endMs: так известные движения остаются точными, а неизвестные не размазываются по прошлому.
func Timeline(now map[string]float64, flows []Flow, endMs int64) ([]Point, map[string]float64) {
	fl := append([]Flow(nil), flows...)
	sort.SliceStable(fl, func(i, j int) bool { return fl[i].T < fl[j].T })

	net := map[string]float64{}
	for _, f := range fl {
		net[f.Asset] += f.Delta
	}
	resid := map[string]float64{}
	for a, v := range now {
		if d := v - net[a]; abs(d) > 1e-9*maxf(1, abs(v)) {
			resid[a] = d
		}
	}
	for a, n := range net {
		if _, ok := now[a]; !ok && abs(n) > 1e-9*maxf(1, abs(n)) {
			resid[a] = -n // движения были, остатка нет: «лишнее» уехало куда-то мимо истории
		}
	}

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
			cur[fl[i].Asset] += fl[i].Delta
			i++
		}
		pts = append(pts, Point{T: t, H: snap()})
	}
	last := map[string]float64{}
	for a, v := range now {
		last[a] = v
	}
	if n := len(pts); n > 0 && pts[n-1].T >= endMs {
		endMs = pts[n-1].T + 1
	}
	pts = append(pts, Point{T: endMs, H: last})
	return pts, resid
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ───────────────────────── загрузка движений ─────────────────────────

const day = 24 * time.Hour

// histHooks — связь восстановления истории с прогрессом и кешем (ставит BuildHistory, пусто — ничего не делаем).
type histHooks struct {
	stage   string
	covered func(stage, key string, fromMs, toMs int64) bool
	start   func(stage string, fromMs, toMs int64)
	done    func(stage, key string, fromMs, toMs int64, fl []Flow, cached bool)
	wait    func(d time.Duration)
}

// eachWindow режет [from,to] на окна не длиннее span (Binance ограничивает длину периода) и идёт по ним от сегодня к прошлому,
// как банки. Сетка окон привязана к from, а не к «сейчас», поэтому в следующий раз окна те же и пропускаются по кешу
// (кроме крайнего справа: его конец — сегодня). Уже сохранённые окна (covered) пропускает; перед окном сообщает «качаю период»,
// после — «период обработан» с найденными движениями (out — накопитель вызывающего: всё, что добавилось за окно, и есть его
// движения). key различает независимые обходы одного этапа (P2P BUY/SELL).
func (c *Client) eachWindow(key string, from, to time.Time, span time.Duration, out *[]Flow, fn func(a, b time.Time) error) error {
	hk := c.hk
	type win struct{ a, b time.Time }
	var grid []win
	for a := from; a.Before(to); a = a.Add(span) {
		b := a.Add(span)
		if b.After(to) {
			b = to
		}
		grid = append(grid, win{a, b})
	}
	for i := len(grid) - 1; i >= 0; i-- {
		a, b := grid[i].a, grid[i].b
		if hk.covered != nil && hk.covered(hk.stage, key, a.UnixMilli(), b.UnixMilli()) {
			if hk.done != nil {
				hk.done(hk.stage, key, a.UnixMilli(), b.UnixMilli(), nil, true)
			}
			continue
		}
		if hk.start != nil {
			hk.start(hk.stage, a.UnixMilli(), b.UnixMilli())
		}
		before := len(*out)
		if err := fn(a, b); err != nil {
			return err
		}
		if hk.done != nil {
			hk.done(hk.stage, key, a.UnixMilli(), b.UnixMilli(), (*out)[before:], false)
		}
	}
	return nil
}

func ms(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }

func skipLD(a string) bool { return strings.HasPrefix(a, "LD") && len(a) > 2 }

func (c *Client) Deposits(from, to time.Time) ([]Flow, error) {
	var out []Flow
	err := c.eachWindow("", from, to, 89*day, &out, func(a, b time.Time) error {
		for off := 0; ; off += 1000 {
			body, err := c.do("GET", "/sapi/v1/capital/deposit/hisrec", url.Values{"startTime": {ms(a)}, "endTime": {ms(b)}, "limit": {"1000"}, "offset": {strconv.Itoa(off)}}, true)
			if err != nil {
				return err
			}
			var r []struct {
				Amount     string
				Coin       string
				Status     int
				InsertTime int64
			}
			if err := json.Unmarshal(body, &r); err != nil {
				return err
			}
			for _, d := range r {
				if d.Status == 1 || d.Status == 6 { // зачислен
					out = append(out, Flow{T: d.InsertTime, Asset: d.Coin, Delta: num(d.Amount), Kind: "deposit"})
				}
			}
			if len(r) < 1000 {
				return nil
			}
		}
	})
	return out, err
}

func (c *Client) Withdrawals(from, to time.Time) ([]Flow, error) {
	var out []Flow
	err := c.eachWindow("", from, to, 89*day, &out, func(a, b time.Time) error {
		for off := 0; ; off += 1000 {
			body, err := c.do("GET", "/sapi/v1/capital/withdraw/history", url.Values{"startTime": {ms(a)}, "endTime": {ms(b)}, "limit": {"1000"}, "offset": {strconv.Itoa(off)}}, true)
			if err != nil {
				return err
			}
			var r []struct {
				Amount         string
				TransactionFee string
				Coin           string
				Status         int
				ApplyTime      string
			}
			if err := json.Unmarshal(body, &r); err != nil {
				return err
			}
			for _, d := range r {
				if d.Status != 6 { // завершён
					continue
				}
				t, err := time.Parse("2006-01-02 15:04:05", d.ApplyTime) // время в UTC
				if err != nil {
					continue
				}
				// списывается сумма вывода; если комиссия сверху, расхождение — доли монеты и уйдёт в «необъяснённое»
				out = append(out, Flow{T: t.UnixMilli(), Asset: d.Coin, Delta: -num(d.Amount), Kind: "withdraw"})
			}
			if len(r) < 1000 {
				return nil
			}
		}
	})
	return out, err
}

func (c *Client) Converts(from, to time.Time) ([]Flow, error) {
	var out []Flow
	err := c.eachWindow("", from, to, 29*day, &out, func(a, b time.Time) error {
		body, err := c.do("GET", "/sapi/v1/convert/tradeFlow", url.Values{"startTime": {ms(a)}, "endTime": {ms(b)}, "limit": {"1000"}}, true)
		if err != nil {
			return err
		}
		var r struct {
			List []struct {
				OrderStatus string
				FromAsset   string
				FromAmount  string
				ToAsset     string
				ToAmount    string
				CreateTime  int64
			} `json:"list"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return err
		}
		for _, d := range r.List {
			if d.OrderStatus != "SUCCESS" {
				continue
			}
			out = append(out,
				Flow{T: d.CreateTime, Asset: d.FromAsset, Delta: -num(d.FromAmount), Kind: "convert"},
				Flow{T: d.CreateTime, Asset: d.ToAsset, Delta: num(d.ToAmount), Kind: "convert"})
		}
		return nil
	})
	return out, err
}

// FiatBuys — покупки криптовалюты картой/банком за фиат.
func (c *Client) FiatBuys(from, to time.Time) ([]Flow, error) {
	var out []Flow
	err := c.eachWindow("", from, to, 29*day, &out, func(a, b time.Time) error {
		for page := 1; ; page++ {
			body, err := c.do("GET", "/sapi/v1/fiat/payments", url.Values{"transactionType": {"0"}, "beginTime": {ms(a)}, "endTime": {ms(b)}, "page": {strconv.Itoa(page)}, "rows": {"500"}}, true)
			if err != nil {
				return err
			}
			var r struct {
				Data []struct {
					CryptoCurrency string
					ObtainAmount   string
					CreateTime     int64
					Status         string
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &r); err != nil {
				return err
			}
			for _, d := range r.Data {
				if d.Status == "Completed" && d.CryptoCurrency != "" {
					out = append(out, Flow{T: d.CreateTime, Asset: d.CryptoCurrency, Delta: num(d.ObtainAmount), Kind: "fiat"})
				}
			}
			if len(r.Data) < 500 {
				return nil
			}
		}
	})
	return out, err
}

// P2P — завершённые сделки P2P (покупка пополняет Funding, продажа списывает).
func (c *Client) P2P(from, to time.Time) ([]Flow, error) {
	var out []Flow
	for _, side := range []string{"BUY", "SELL"} {
		sign := 1.0
		if side == "SELL" {
			sign = -1
		}
		err := c.eachWindow(side, from, to, 29*day, &out, func(a, b time.Time) error {
			for page := 1; ; page++ {
				body, err := c.do("GET", "/sapi/v1/c2c/orderMatch/listUserOrderHistory", url.Values{"tradeType": {side}, "startTimestamp": {ms(a)}, "endTimestamp": {ms(b)}, "page": {strconv.Itoa(page)}, "rows": {"100"}}, true)
				if err != nil {
					return err
				}
				var r struct {
					Data []struct {
						Asset       string
						Amount      string
						OrderStatus string
						CreateTime  int64
					} `json:"data"`
				}
				if err := json.Unmarshal(body, &r); err != nil {
					return err
				}
				for _, d := range r.Data {
					if d.OrderStatus == "COMPLETED" {
						out = append(out, Flow{T: d.CreateTime, Asset: d.Asset, Delta: sign * num(d.Amount), Kind: "p2p"})
					}
				}
				if len(r.Data) < 100 {
					return nil
				}
			}
		})
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// Dividends — начисления (ставки, раздачи, награды), которые Binance отдаёт как asset dividend.
func (c *Client) Dividends(from, to time.Time) ([]Flow, error) {
	var out []Flow
	err := c.eachWindow("", from, to, 89*day, &out, func(a, b time.Time) error {
		body, err := c.do("GET", "/sapi/v1/asset/assetDividend", url.Values{"startTime": {ms(a)}, "endTime": {ms(b)}, "limit": {"500"}}, true)
		if err != nil {
			return err
		}
		var r struct {
			Rows []struct {
				Amount  string
				Asset   string
				DivTime int64
			} `json:"rows"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return err
		}
		for _, d := range r.Rows {
			out = append(out, Flow{T: d.DivTime, Asset: d.Asset, Delta: num(d.Amount), Kind: "dividend"})
		}
		return nil
	})
	return out, err
}

// Sym — торговая пара.
type Sym struct{ Base, Quote string }

// Symbols — все пары биржи: symbol → (base, quote).
func (c *Client) Symbols() (map[string]Sym, error) {
	c.mu.Lock()
	cached := c.syms
	c.mu.Unlock()
	if cached != nil {
		return cached, nil
	}
	out, err := c.loadSymbols()
	if err == nil {
		c.mu.Lock()
		c.syms = out
		c.mu.Unlock()
	}
	return out, err
}

func (c *Client) loadSymbols() (map[string]Sym, error) {
	body, err := c.do("GET", "/api/v3/exchangeInfo", nil, false)
	if err != nil {
		return nil, err
	}
	var r struct {
		Symbols []struct{ Symbol, BaseAsset, QuoteAsset string } `json:"symbols"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	out := make(map[string]Sym, len(r.Symbols))
	for _, s := range r.Symbols {
		out[s.Symbol] = Sym{s.BaseAsset, s.QuoteAsset}
	}
	return out, nil
}

// Trades — все спот-сделки по паре (постранично с самой первой).
func (c *Client) Trades(symbol string, s Sym) ([]Flow, error) {
	var out []Flow
	from := int64(0)
	for {
		body, err := c.do("GET", "/api/v3/myTrades", url.Values{"symbol": {symbol}, "fromId": {strconv.FormatInt(from, 10)}, "limit": {"1000"}}, true)
		if err != nil {
			return out, err
		}
		var r []struct {
			ID              int64
			Qty             string
			QuoteQty        string
			Commission      string
			CommissionAsset string
			Time            int64
			IsBuyer         bool
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return out, err
		}
		for _, t := range r {
			base, quote := num(t.Qty), num(t.QuoteQty)
			if t.IsBuyer {
				out = append(out, Flow{T: t.Time, Asset: s.Base, Delta: base, Kind: "trade"}, Flow{T: t.Time, Asset: s.Quote, Delta: -quote, Kind: "trade"})
			} else {
				out = append(out, Flow{T: t.Time, Asset: s.Base, Delta: -base, Kind: "trade"}, Flow{T: t.Time, Asset: s.Quote, Delta: quote, Kind: "trade"})
			}
			if cm := num(t.Commission); cm != 0 && t.CommissionAsset != "" {
				out = append(out, Flow{T: t.Time, Asset: t.CommissionAsset, Delta: -cm, Kind: "fee"})
			}
			from = t.ID + 1
		}
		if len(r) < 1000 {
			return out, nil
		}
	}
}

// ───────────────────────── минутные цены ─────────────────────────

const minute = int64(60_000)

// klines тянет 1m-свечи пары на [fromMs,toMs] и отдаёт (время открытия, закрытие).
func (c *Client) klines(symbol string, fromMs, toMs int64, put func(t int64, closeP float64)) error {
	start := fromMs
	for start <= toMs {
		if c.hk.start != nil {
			c.hk.start(c.hk.stage, start, min64(toMs, start+1000*minute))
		}
		body, err := c.do("GET", "/api/v3/klines", url.Values{"symbol": {symbol}, "interval": {"1m"}, "startTime": {strconv.FormatInt(start, 10)}, "endTime": {strconv.FormatInt(toMs, 10)}, "limit": {"1000"}}, false)
		if err != nil {
			return err
		}
		var r [][]json.RawMessage
		if err := json.Unmarshal(body, &r); err != nil {
			return err
		}
		if len(r) == 0 {
			return nil
		}
		var last int64
		if c.hk.done != nil {
			c.hk.done(c.hk.stage, "", start, start+int64(len(r))*minute, nil, false)
		}
		for _, k := range r {
			if len(k) < 5 {
				continue
			}
			var t float64
			var cl string
			_ = json.Unmarshal(k[0], &t)
			_ = json.Unmarshal(k[4], &cl)
			last = int64(t)
			put(last, num(cl))
		}
		if len(r) < 1000 {
			return nil
		}
		start = last + minute
	}
	return nil
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
	for k, a := range names {
		if progress != nil {
			progress(k, len(names))
		}
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
	if progress != nil {
		progress(len(names), len(names))
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

// ───────────────────────── сборка всей истории ─────────────────────────

// HistoryOpts — параметры восстановления.
type HistoryOpts struct {
	From     time.Time                            // как далеко в прошлое искать движения
	Now      time.Time                            // конец истории (по умолчанию — сейчас)
	Progress func(stage string, done, total int)  // для окна прогресса
	Period   func(firstMs int64, flowsMs []int64) // когда движения найдены: с какой даты реально идёт история и когда были движения
	Quotes   []string                             // котировочные монеты, по парам с которыми ищем сделки (по умолчанию стейблы/BTC/ETH/BNB)

	// Кеш периодов: каждый просмотренный период сразу уходит наружу (Done) и в следующий раз пропускается (Covered).
	Covered func(stage, key string, fromMs, toMs int64) bool                    // период уже в кеше — не запрашивать
	Start   func(stage string, fromMs, toMs int64)                              // период, который сейчас запрашивается
	Done    func(stage, key string, fromMs, toMs int64, fl []Flow, cached bool) // период обработан (fl — найденные в нём движения; cached — взят из кеша, не запрашивался)
	Wait    func(d time.Duration)                                               // вынужденная пауза из-за лимита (0 — закончилась)
	Cached  []Flow                                                              // движения из пропущенных (уже сохранённых) периодов
}

// History — результат восстановления.
type History struct {
	Series    *SeriesResult
	Flows     int
	FlowTimes []int64 // моменты движений (мс), по возрастанию
	Warnings  []string
	Residual  map[string]float64 // монета → остаток − Σдвижений (то, что API не объяснил)
	Assets    []string
}

var defaultQuotes = []string{"USDT", "USDC", "FDUSD", "BTC", "ETH", "BNB", "EUR", "UAH"}

var extLabels = map[string]string{"deposit": "депозит", "withdraw": "вывод", "fiat": "покупка картой", "p2p": "P2P", "dividend": "начисление"}

// ExtLabel — подпись внешнего движения (депозит, вывод, …); ok == false у внутренних (сделки, комиссии, конвертации).
func ExtLabel(kind string) (string, bool) { l, ok := extLabels[kind]; return l, ok }

// ExtKinds — виды внешних движений, которые становятся операциями счёта.
func ExtKinds() []string { return []string{"deposit", "withdraw", "fiat", "p2p", "dividend"} }

// flowKey — отпечаток движения (для слияния кеша с заново запрошенным хвостом).
func flowKey(f Flow) string { return fmt.Sprintf("%d|%s|%.12g|%s", f.T, f.Asset, f.Delta, f.Kind) }

// BuildHistory собирает движения, откатывает остатки назад и оценивает их по минутам.
func (c *Client) BuildHistory(o HistoryOpts) (*History, error) {
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	prog := func(stage string, d, t int) {
		if o.Progress != nil {
			o.Progress(stage, d, t)
		}
	}
	c.hk = histHooks{covered: o.Covered, start: o.Start, done: o.Done, wait: o.Wait}
	defer func() { c.hk = histHooks{} }()
	_ = c.SyncTime()
	prog("остатки", 0, 1)
	h, warns, err := c.Fetch()
	if err != nil {
		return nil, err
	}
	nowTotals := map[string]float64{}
	for a, w := range h {
		if t := w.Total(); t != 0 {
			nowTotals[a] = t
		}
	}

	// кеш: счётчики уже известных движений; свежие дописываем только сверх них (хвост окна запрашивается заново целиком)
	have := map[string]int{}
	flows := append([]Flow(nil), o.Cached...)
	for _, f := range o.Cached {
		have[flowKey(f)]++
	}
	add := func(fl []Flow) {
		for _, f := range fl {
			if k := flowKey(f); have[k] > 0 {
				have[k]--
				continue
			}
			flows = append(flows, f)
		}
	}
	collect := func(name string, f func() ([]Flow, error)) {
		c.hk.stage = name
		prog(name, 0, 1)
		got, err := f()
		add(got)
		if err != nil {
			warns = append(warns, name+": "+err.Error())
		}
	}
	collect("депозиты", func() ([]Flow, error) { return c.Deposits(o.From, o.Now) })
	collect("выводы", func() ([]Flow, error) { return c.Withdrawals(o.From, o.Now) })
	collect("конвертации", func() ([]Flow, error) { return c.Converts(o.From, o.Now) })
	collect("покупки картой", func() ([]Flow, error) { return c.FiatBuys(o.From, o.Now) })
	collect("P2P", func() ([]Flow, error) { return c.P2P(o.From, o.Now) })
	collect("начисления", func() ([]Flow, error) { return c.Dividends(o.From, o.Now) })

	syms, err := c.Symbols()
	if err != nil {
		return nil, err
	}

	// сделки: пары, в которых участвуют известные нам монеты, плюс все пары к USDT
	known := map[string]bool{}
	for a := range nowTotals {
		known[a] = true
	}
	for _, f := range flows {
		known[f.Asset] = true
	}
	quotes := o.Quotes
	if quotes == nil {
		quotes = defaultQuotes
	}
	isQ := map[string]bool{}
	for _, q := range quotes {
		isQ[q] = true
	}
	var scan []string
	for sym, s := range syms {
		okBase := known[s.Base] || isQ[s.Base]
		okQuote := known[s.Quote] || isQ[s.Quote]
		if (known[s.Base] || known[s.Quote]) && okBase && okQuote || s.Quote == "USDT" {
			scan = append(scan, sym)
		}
	}
	sort.Strings(scan)
	c.hk.stage = "сделки"
	for i, sym := range scan {
		prog("сделки", i, len(scan))
		if o.Covered != nil && o.Covered("сделки", sym, 0, 0) { // пара уже просмотрена и лежит в кеше
			continue
		}
		tr, err := c.Trades(sym, syms[sym])
		add(tr)
		if err != nil {
			warns = append(warns, "сделки "+sym+": "+err.Error())
		} else if o.Done != nil {
			o.Done("сделки", sym, 0, o.Now.UnixMilli(), tr, false)
		}
		c.sleep(120 * time.Millisecond) // myTrades весит 20, лимит 6000 в минуту на IP
	}
	prog("сделки", len(scan), len(scan))

	// конечные точки: движения за пределами периода нам не нужны, лишние монеты LD* — зеркала Earn
	var use []Flow
	for _, f := range flows {
		if f.Delta == 0 || skipLD(f.Asset) || f.T > o.Now.UnixMilli() {
			continue
		}
		use = append(use, f)
	}
	res := &History{Flows: len(use), Warnings: warns}
	for _, f := range use {
		res.FlowTimes = append(res.FlowTimes, f.T)
	}
	sort.Slice(res.FlowTimes, func(i, j int) bool { return res.FlowTimes[i] < res.FlowTimes[j] })
	if len(use) == 0 {
		res.Warnings = append(res.Warnings, "движений по счёту не найдено — истории нет")
		return res, nil
	}
	pts, resid := Timeline(nowTotals, use, o.Now.UnixMilli())
	res.Residual = resid
	if o.Period != nil {
		o.Period(res.FlowTimes[0], res.FlowTimes)
	}
	for a := range known {
		res.Assets = append(res.Assets, a)
	}
	sort.Strings(res.Assets)

	g0 := pts[0].T / minute * minute
	end := o.Now.UnixMilli() / minute * minute
	c.hk.stage = "цены"
	s, err := c.Series(pts, g0, end, syms, func(d, t int) { prog("цены", d, t) })
	if err != nil {
		return nil, err
	}
	res.Series = s
	if len(s.Unpriced) > 0 {
		res.Warnings = append(res.Warnings, "нет минутных цен: "+strings.Join(s.Unpriced, ", "))
	}
	return res, nil
}

// ExtendSeries дописывает ряд от fromMs до endMs при неизменных остатках cur (пока нет новых движений, они те же).
func (c *Client) ExtendSeries(cur Holdings, fromMs, endMs int64) (*SeriesResult, error) {
	syms, err := c.Symbols()
	if err != nil {
		return nil, err
	}
	m := map[string]float64{}
	for a, w := range cur {
		if t := w.Total(); t != 0 {
			m[a] = t
		}
	}
	g0 := fromMs / minute * minute
	end := endMs / minute * minute
	if end < g0 {
		return &SeriesResult{StartMs: g0}, nil
	}
	return c.Series([]Point{{T: g0, H: m}}, g0, end, syms, nil)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

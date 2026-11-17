package binance

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Гривневый курс USDT. Пока пара USDTUAH торгуется, курс берётся из её минутных свечей. Binance снял пару с торгов
// (последняя свеча — 12.11.2026): после последней свечи курс берётся из внешнего источника Client.UAHRate (официальный
// курс доллара НБУ, по дням), а если его нет — остаётся ценой последней свечи. Правило одно для ряда, оценки движений
// и текущей оценки портфеля: «пара жива на минуту t» значит, что у неё есть свеча на эту минуту или позже.
const rateSym = "USDTUAH"

// candle — одна минутная свеча пары у границы t: param "endTime" — последняя не позже t, "startTime" — первая не раньше t.
func (c *Client) candle(symbol, param string, t int64) (openMs int64, closeP float64, ok bool, err error) {
	body, err := c.do("/api/v3/klines", url.Values{"symbol": {symbol}, "interval": {"1m"}, param: {strconv.FormatInt(t, 10)}, "limit": {"1"}})
	if err != nil {
		return 0, 0, false, err
	}
	var r [][]json.RawMessage
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, 0, false, err
	}
	if len(r) == 0 || len(r[0]) < 5 {
		return 0, 0, false, nil
	}
	var tf float64
	var s string
	_ = json.Unmarshal(r[0][0], &tf)
	_ = json.Unmarshal(r[0][4], &s)
	p := num(s)
	return int64(tf), p, p > 0, nil
}

func (c *Client) external(tMs int64) float64 {
	if c.UAHRate == nil {
		return 0
	}
	return c.UAHRate(tMs)
}

// RateUAH — гривен за 1 USDT на момент tMs и признак external: курс взят не из свечей пары, а из внешнего источника
// (пара на этот момент уже не торгуется).
func (c *Client) RateUAH(tMs int64) (rate float64, external bool, err error) {
	t := tMs / minute * minute
	before, prev, okPrev, err := c.candle(rateSym, "endTime", t)
	if err != nil {
		return 0, false, err
	}
	if okPrev && before >= t-minute { // свеча этой минуты (или предыдущей: текущая могла ещё не открыться)
		return prev, false, nil
	}
	_, next, okNext, err := c.candle(rateSym, "startTime", t)
	if err != nil {
		return 0, false, err
	}
	switch {
	case okNext && okPrev: // дыра в торгах живой пары: цена последней сделки
		return prev, false, nil
	case okNext: // до первой свечи пары: первая известная цена
		return next, false, nil
	}
	if v := c.external(t); v > 0 {
		return v, true, nil
	}
	if okPrev { // внешнего курса нет: последняя цена пары
		return prev, false, nil
	}
	return 0, false, errNoRate
}

// rateGrid — курс USDT/UAH на минутной сетке [g0, g0+n*minute). Внутри торгов пропуски заполняются прошлой ценой,
// начало — первой известной; минуты после последней свечи пары — внешним курсом (см. RateUAH).
func (c *Client) rateGrid(g0 int64, n int) ([]float64, error) {
	px := make([]float64, n)
	err := c.klines(rateSym, g0, g0+int64(n)*minute-1, func(t int64, p float64) {
		if i := int((t - g0) / minute); i >= 0 && i < n && p > 0 {
			px[i] = p
		}
	})
	if err != nil {
		return nil, err
	}
	first, last := -1, -1
	for i, p := range px {
		if p > 0 {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	// Хвост без свечей длиннее одной минуты (текущая могла ещё не открыться) — пара уже не торгуется.
	dead := n // с какой минуты сетки пара мертва
	if last < n-2 {
		dead = last + 1
	}
	if first < 0 { // в окне ни одной свечи: опора — последняя цена пары до окна, если она была
		_, prev, ok, err := c.candle(rateSym, "endTime", g0)
		if err != nil {
			return nil, err
		}
		carry := 0.0
		if ok {
			carry = prev
		}
		for i := range px {
			if v := c.external(g0 + int64(i)*minute); v > 0 {
				carry = v
			}
			if carry <= 0 {
				return nil, errNoRate
			}
			px[i] = carry
		}
		return px, nil
	}
	for i := 0; i < first; i++ {
		px[i] = px[first]
	}
	for i := first + 1; i < n; i++ {
		if px[i] > 0 {
			continue
		}
		px[i] = px[i-1]
		if i >= dead {
			if v := c.external(g0 + int64(i)*minute); v > 0 {
				px[i] = v
			}
		}
	}
	return px, nil
}

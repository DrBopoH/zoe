package market

import (
	"sort"
	"time"
)

// SeriesStat — что лежит в кеше по одному ряду (считает хранилище): число точек, первая и последняя дата, когда ряд обновлялся.
type SeriesStat struct {
	ID        string `json:"id"`
	Points    int    `json:"points"`
	From      string `json:"from"`
	To        string `json:"to"`
	UpdatedAt int64  `json:"updated_at"`
}

// SeriesHealth — ряд с оценкой свежести: частота по среднему шагу точек и отставание последней точки от сегодня.
type SeriesHealth struct {
	SeriesStat
	Feed    string `json:"feed"`
	Freq    string `json:"freq"`
	LagDays int    `json:"lag_days"`
	Stale   bool   `json:"stale"`
}

// FeedHealth — итог по источнику: сколько рядов он обещает и сколько есть в кеше, последняя точка, устаревшие ряды, последняя ошибка.
type FeedHealth struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Series    int    `json:"series"`
	Cached    int    `json:"cached"`
	Points    int    `json:"points"`
	LastDate  string `json:"last_date,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
	Stale     int    `json:"stale"`
	Error     string `json:"error,omitempty"`
}

// FreqInfo — частота ряда словами и срок, после которого ряд считается устаревшим (с запасом на задержку публикации: инфляция выходит через месяц-полтора).
type FreqInfo struct {
	Label     string
	StaleDays int
}

// Frequency определяет частоту по среднему шагу между точками. Меньше двух точек — «снимок» (цена в момент опроса): его возраст считается по updated_at.
func Frequency(points int, from, to string) FreqInfo {
	if points < 2 {
		return FreqInfo{"снимок", 3}
	}
	a, errA := time.Parse("2006-01-02", from)
	b, errB := time.Parse("2006-01-02", to)
	if errA != nil || errB != nil || !b.After(a) {
		return FreqInfo{"снимок", 3}
	}
	step := b.Sub(a).Hours() / 24 / float64(points-1)
	switch {
	case step <= 3:
		return FreqInfo{"день", 7}
	case step <= 10:
		return FreqInfo{"неделя", 21}
	case step <= 45:
		return FreqInfo{"месяц", 150}
	case step <= 120:
		return FreqInfo{"квартал", 270}
	}
	return FreqInfo{"год", 900}
}

// Сроки считаются от даты точки, а точка стоит на начале периода (месяц — 1-е число, квартал — первый день, год — середина): к концу периода уже
// проходит месяц-квартал, плюс задержка публикации (ВВП и баланс ФРС — до полугода, мировые данные ОЭСР и Всемирного банка — до полутора лет).
// Запасы подобраны по живой странице «Источников» (17.11.2026): ряды с обычной задержкой не краснеют, а те, что отстали на месяцы сверх неё, — да.

// stepRates — ключевые ставки стран: ряд-«ступенька», точка есть только в день смены ставки, и «последняя точка 14 месяцев назад» значит «ставку не меняли»,
// а не «данные устарели» (Китай держал ставку с мая 2025). Список берётся из реестра стран, отдельно его никто не ведёт.
func stepRates() map[string]bool {
	m := map[string]bool{}
	for _, c := range Countries() {
		if id := c.Metrics["policy"]; id != "" {
			m[id] = true
		}
	}
	return m
}

// stepStaleDays — через сколько дней после последнего опроса ряд-ступенька считается устаревшим (опрос заменяет ряд целиком, так что это возраст кеша).
const stepStaleDays = 45

// Health сводит каталог источников и статистику кеша в здоровье источников и рядов. errs — последняя ошибка каждого источника (id → текст).
// Источники без Provides (облигации) пропускаются. Ряды в кеше, которых нет ни в одном источнике, идут с пустым Feed.
func Health(feeds []Feed, stats []SeriesStat, errs map[string]string, now time.Time) ([]FeedHealth, []SeriesHealth) {
	byID := map[string]SeriesStat{}
	for _, s := range stats {
		byID[s.ID] = s
	}
	steps := stepRates()
	owner := map[string]string{}
	var fh []FeedHealth
	var sh []SeriesHealth
	for _, f := range feeds {
		if f.Provides == nil {
			continue
		}
		h := FeedHealth{ID: f.ID, Title: f.Title, Series: len(f.Provides), Error: errs[f.ID]}
		for _, id := range f.Provides {
			st, ok := byID[id]
			if !ok {
				continue
			}
			if _, dup := owner[id]; dup {
				continue
			}
			owner[id] = f.ID
			x := seriesHealth(st, f.ID, now, steps[id])
			sh = append(sh, x)
			h.Cached++
			h.Points += st.Points
			if x.Stale {
				h.Stale++
			}
			if st.To > h.LastDate {
				h.LastDate = st.To
			}
			if st.UpdatedAt > h.UpdatedAt {
				h.UpdatedAt = st.UpdatedAt
			}
		}
		fh = append(fh, h)
	}
	for _, st := range stats {
		if _, ok := owner[st.ID]; !ok {
			sh = append(sh, seriesHealth(st, "", now, steps[st.ID]))
		}
	}
	sort.SliceStable(sh, func(i, j int) bool { return sh[i].ID < sh[j].ID })
	return fh, sh
}

func seriesHealth(st SeriesStat, feed string, now time.Time, step bool) SeriesHealth {
	fi := Frequency(st.Points, st.From, st.To)
	if step && st.Points > 0 {
		fi = FreqInfo{"по решению", stepStaleDays}
	}
	var lag int
	if fi.Label == "снимок" || fi.Label == "по решению" {
		if st.UpdatedAt > 0 {
			lag = int(now.Sub(time.Unix(st.UpdatedAt, 0)).Hours() / 24)
		}
	} else if t, err := time.Parse("2006-01-02", st.To); err == nil {
		lag = int(now.Sub(t).Hours() / 24)
	}
	if lag < 0 {
		lag = 0
	}
	return SeriesHealth{SeriesStat: st, Feed: feed, Freq: fi.Label, LagDays: lag, Stale: lag > fi.StaleDays}
}

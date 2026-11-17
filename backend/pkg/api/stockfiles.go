package api

import (
	"math"
	"sort"
	"strings"
	"time"

	"economic/pkg/market"
	"economic/pkg/source"
)

// Данные раздела «Акции» (цены, сплиты, число акций из отчётности SEC) лежат в двух местах, как макро-ряды: кеш (cache.db) и файлы группы «Экономика»
// (наборы stock-prices и stock-facts). Из файлов кеш собирается заново (RestoreStocks), без новых запросов к Yahoo и EDGAR: это около 6 минут ожидания.
// Список компаний в файлы не идёт: Nasdaq и SEC отдают его за секунду, и у него есть смысл только свежим.
//
// Файл набора переписывается целиком, поэтому пишем пачками: каждый тикер сразу в кеш, а в файл — раз в stockFlushEvery тикеров и в конце опроса.
//
// Раскладка рядов набора stock-prices: «AAPL» — закрытие месяца (время — первое число месяца), «AAPL:a» — с дивидендами, «AAPL:s» — сплиты (время — день, значение — во сколько раз).
// Набор stock-facts: на каждый кадр SEC два ряда с одним временем: «тег|период» — число (время — дата отчёта у компании) и «тег|период|cik» — CIK компании.

const (
	stockFlushEvery  = 25 // тикеров в пачке файла цен
	stockFlushFrames = 20 // кадров в пачке файла отчётности
)

// stockBuf копит ряды для файла набора и сбрасывает их пачками.
type stockBuf struct {
	card  string
	every int
	set   map[string][]source.Tick
	n     int
}

func newStockBuf(card string, every int) *stockBuf {
	return &stockBuf{card: card, every: every, set: map[string][]source.Tick{}}
}

// add кладёт ряды одной единицы (тикера, кадра); пачка полна — сбрасывает в файл.
func (b *stockBuf) add(series map[string][]source.Tick) error {
	for id, t := range series {
		b.set[id] = t
	}
	if b.n++; b.n >= b.every {
		return b.flush()
	}
	return nil
}

func (b *stockBuf) flush() error {
	if len(b.set) == 0 {
		return nil
	}
	err := store().PutSeries(b.card, b.set)
	if err == nil {
		b.set, b.n = map[string][]source.Tick{}, 0
	}
	return err
}

func dayTick(date string, v float64) (source.Tick, bool) {
	d, err := time.Parse(dayLayout, date)
	return source.Tick{T: d.Unix(), V: v}, err == nil
}

// pricesSaver — функции опроса цен: save кладёт тикер в кеш и в буфер файла, done сбрасывает остаток (вызывается и после отмены опроса).
func (s *Server) pricesSaver(now int64) (save func(string, []market.PriceBar, []market.Split) error, done func(res *market.Result)) {
	buf := newStockBuf(source.StockPricesCard, stockFlushEvery)
	var fileErr error
	save = func(tk string, bars []market.PriceBar, splits []market.Split) error {
		if err := s.storage.SaveStockPrices(tk, bars, splits, now); err != nil {
			return err
		}
		tk = strings.ToUpper(tk)
		series := map[string][]source.Tick{}
		for _, b := range bars {
			if t, ok := dayTick(b.Date, b.Close); ok {
				series[tk] = append(series[tk], t)
				a, _ := dayTick(b.Date, b.Adj)
				series[tk+":a"] = append(series[tk+":a"], a)
			}
		}
		for _, sp := range splits {
			if t, ok := dayTick(sp.Date, sp.Ratio); ok {
				series[tk+":s"] = append(series[tk+":s"], t)
			}
		}
		if err := buf.add(series); err != nil && fileErr == nil { // сбой файла не должен ронять опрос: кеш уже сохранён
			fileErr = err
		}
		return nil
	}
	done = func(res *market.Result) {
		if err := buf.flush(); err != nil && fileErr == nil {
			fileErr = err
		}
		if fileErr != nil {
			res.Error = strings.TrimSpace(res.Error + " файл цен: " + fileErr.Error())
		}
	}
	return
}

// factsSaver — то же для кадров отчётности.
func (s *Server) factsSaver(now int64) (save func(market.FrameRequest, []market.FrameFact) error, done func(res *market.Result)) {
	buf := newStockBuf(source.StockFactsCard, stockFlushFrames)
	var fileErr error
	save = func(r market.FrameRequest, facts []market.FrameFact) error {
		if err := s.storage.SaveStockFrame(r, facts, now); err != nil {
			return err
		}
		sorted := append([]market.FrameFact(nil), facts...)
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].End < sorted[j].End })
		vals, ciks := make([]source.Tick, 0, len(sorted)), make([]source.Tick, 0, len(sorted))
		for _, f := range sorted {
			if t, ok := dayTick(f.End, f.Val); ok {
				vals = append(vals, t)
				ciks = append(ciks, source.Tick{T: t.T, V: float64(f.CIK)})
			}
		}
		if err := buf.add(map[string][]source.Tick{r.Key(): vals, r.Key() + "|cik": ciks}); err != nil && fileErr == nil {
			fileErr = err
		}
		return nil
	}
	done = func(res *market.Result) {
		if err := buf.flush(); err != nil && fileErr == nil {
			fileErr = err
		}
		if fileErr != nil {
			res.Error = strings.TrimSpace(res.Error + " файл отчётности: " + fileErr.Error())
		}
	}
	return
}

// RestoreStocks собирает кеш акций из файлов группы «Экономика»: добавляет только тикеры и кадры, которых в кеше нет (кеш не перетирается).
// Время загрузки у восстановленного — создание самого свежего файла набора: дальше обычное устаревание (история 20 часов, кадры свежих кварталов 3 дня).
// Возвращает, сколько тикеров и кадров восстановлено.
func (s *Server) RestoreStocks() (tickers, frames int, err error) {
	haveT, err := s.storage.StockFetchedAt()
	if err != nil {
		return 0, 0, err
	}
	haveF, err := s.storage.StockFramesHave()
	if err != nil {
		return 0, 0, err
	}
	prices, at := store().LoadSets(source.StockPricesCard)
	for id, ticks := range prices {
		if strings.Contains(id, ":") || len(ticks) == 0 {
			continue
		}
		if _, ok := haveT[id]; ok {
			continue
		}
		adj := prices[id+":a"]
		bars := make([]market.PriceBar, 0, len(ticks))
		for i, t := range ticks {
			b := market.PriceBar{Ticker: id, Date: time.Unix(t.T, 0).UTC().Format(dayLayout), Close: t.V, Adj: t.V}
			if len(adj) == len(ticks) && adj[i].V > 0 {
				b.Adj = adj[i].V
			}
			bars = append(bars, b)
		}
		var splits []market.Split
		for _, t := range prices[id+":s"] {
			splits = append(splits, market.Split{Date: time.Unix(t.T, 0).UTC().Format(dayLayout), Ratio: t.V})
		}
		if err := s.storage.SaveStockPrices(id, bars, splits, at); err != nil {
			return tickers, frames, err
		}
		tickers++
	}
	facts, at := store().LoadSets(source.StockFactsCard)
	for key, vals := range facts {
		tag, period, ok := strings.Cut(key, "|")
		if !ok || strings.Contains(period, "|") {
			continue // это ряд CIK: он разбирается вместе со своим кадром
		}
		if _, have := haveF[key]; have {
			continue
		}
		ciks := facts[key+"|cik"]
		if len(ciks) != len(vals) {
			continue // файл повреждён: кадр скачается заново
		}
		list := make([]market.FrameFact, 0, len(vals))
		for i, v := range vals {
			if c := ciks[i].V; c > 0 && c < math.MaxInt32 {
				list = append(list, market.FrameFact{CIK: int(c), End: time.Unix(v.T, 0).UTC().Format(dayLayout), Val: v.V})
			}
		}
		if err := s.storage.SaveStockFrame(market.FrameRequest{Tag: tag, Period: period}, list, at); err != nil {
			return tickers, frames, err
		}
		frames++
	}
	return tickers, frames, nil
}

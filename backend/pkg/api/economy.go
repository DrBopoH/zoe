package api

import (
	"sort"
	"time"

	"economic/pkg/market"
	"economic/pkg/source"
)

// Макро-ряды лежат в двух местах: кеш (cache.db — быстро читать экранам) и файлы группы «Экономика» (источник истины, из него кеш собирается).
// Файл на источник: economy/macro-<источник>/. Часовые снимки крипто-контура — отдельный набор chain-ticks (handlers_market.go).

const dayLayout = "2006-01-02"

// persistSeries кладёт в файл набора источника feed ряды, пришедшие в опросе: берутся из кеша целиком (докачанный хвост уже склеен с историей).
// Снимки (P2P, комиссии, доминация) сюда не идут — их история в chain-ticks.
func (s *Server) persistSeries(feed string, pts []market.SeriesPoint) error {
	var ids []string
	seen := map[string]bool{}
	for _, p := range pts {
		if !seen[p.Series] && !market.IsSnapshot(p.Series) {
			seen[p.Series] = true
			ids = append(ids, p.Series)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	got, _, err := s.storage.MacroSeries(ids)
	if err != nil {
		return err
	}
	set := make(map[string][]source.Tick, len(got))
	for id, list := range got {
		ticks := make([]source.Tick, 0, len(list))
		for _, p := range list {
			if d, err := time.Parse(dayLayout, p.Date); err == nil {
				ticks = append(ticks, source.Tick{T: d.Unix(), V: p.Value})
			}
		}
		set[id] = ticks
	}
	return store().PutSeries(source.MacroCard(feed), set)
}

// RestoreEconomy собирает кеш макро-рядов из файлов группы «Экономика»: добавляет только те ряды, которых в кеше нет (свежее скачанное не
// перетирается). Снимки восстанавливаются из часовых точек — последним значением за день. Возвращает, сколько рядов добавлено.
func (s *Server) RestoreEconomy() (int, error) {
	stats, err := s.storage.MacroStats()
	if err != nil {
		return 0, err
	}
	have := make(map[string]bool, len(stats))
	for _, st := range stats {
		have[st.ID] = true
	}
	files, created := store().LoadSeries()
	var pts []market.SeriesPoint
	restored := 0
	add := func(id string, ticks []source.Tick) {
		if len(ticks) == 0 {
			return
		}
		restored++
		perDay := map[string]float64{}
		for _, t := range ticks {
			perDay[time.Unix(t.T, 0).UTC().Format(dayLayout)] = t.V // ряд по возрастанию: последнее значение дня побеждает
		}
		days := make([]string, 0, len(perDay))
		for d := range perDay {
			days = append(days, d)
		}
		sort.Strings(days)
		for _, d := range days {
			pts = append(pts, market.SeriesPoint{Series: id, Date: d, Value: perDay[d]})
		}
	}
	for id, ticks := range files {
		if !have[id] && !market.IsSnapshot(id) {
			add(id, ticks)
		}
	}
	for _, id := range market.SnapshotSeries {
		if !have[id] {
			add(id, store().ReadTicks(source.SetChainTicks, id, 0))
		}
	}
	if len(pts) == 0 {
		return 0, nil
	}
	if created == 0 {
		created = time.Now().Unix()
	}
	return restored, s.storage.SaveMacroSeries(pts, created)
}

package db

import (
	"sort"
	"strings"

	"economic/pkg/market"
)

// Макро-ряды НБУ (ставки, инфляция) — кеш: стереть и скачать заново можно в любой момент.
const macroCacheSchema = `
CREATE TABLE IF NOT EXISTS macro_series (
	series TEXT NOT NULL,           -- policy_rate | overnight_cd | cpi_yoy | …
	dt TEXT NOT NULL,               -- YYYY-MM-DD
	value REAL NOT NULL,
	updated_at INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (series, dt)
);`

// SaveMacroSeries заменяет целиком каждый ряд, который есть в списке (скачанный ряд полнее и свежее старого);
// ряды, которых в списке нет, не трогаются — сбой одной части источника не стирает другую.
func (s *Storage) SaveMacroSeries(pts []market.SeriesPoint, nowUnix int64) error {
	if len(pts) == 0 {
		return nil
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	del, err := tx.Prepare(`DELETE FROM macro_series WHERE series = ?`)
	if err != nil {
		return err
	}
	defer del.Close()
	seen := map[string]bool{}
	for _, p := range pts {
		if p.Series == "" || p.Date == "" || seen[p.Series] {
			continue
		}
		seen[p.Series] = true
		if _, err := del.Exec(p.Series); err != nil {
			return err
		}
	}
	// подготовленный запрос: ряды на десятки тысяч точек (VIX, спред, S&P) не разбираются заново на каждой строке
	ins, err := tx.Prepare(`INSERT OR REPLACE INTO macro_series(series, dt, value, updated_at) VALUES(?,?,?,?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, p := range pts {
		if p.Series == "" || p.Date == "" {
			continue
		}
		if _, err := ins.Exec(p.Series, p.Date, p.Value, nowUnix); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// maxMacroIDs — сколько рядов за раз можно отобрать в SQL; дальше читается всё и отбирается в Go.
const maxMacroIDs = 200

// MacroPoint — точка ряда для JSON.
type MacroPoint struct {
	Date  string  `json:"d"`
	Value float64 `json:"v"`
}

// MacroSeries — ряды по возрастанию даты (ids пустой — все) и время последнего обновления.
func (s *Storage) MacroSeries(ids []string) (map[string][]MacroPoint, int64, error) {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	// фильтр в SQL: раньше читалась вся таблица (десятки тысяч точек всех экранов), а нужные ряды отбирались уже в Go
	q, args := `SELECT series, dt, value, updated_at FROM macro_series`, []any{}
	if len(ids) > 0 && len(ids) <= maxMacroIDs { // больше — это «всё»: у SQLite лимит переменных запроса
		marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		q += ` WHERE series IN (` + marks + `)`
		for _, id := range ids {
			args = append(args, id)
		}
	}
	rows, err := s.cacheDB.Query(q+` ORDER BY series, dt`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := map[string][]MacroPoint{}
	var updated int64
	for rows.Next() {
		var id, dt string
		var v float64
		var at int64
		if err := rows.Scan(&id, &dt, &v, &at); err != nil {
			return nil, 0, err
		}
		if len(want) > 0 && !want[id] {
			continue
		}
		out[id] = append(out[id], MacroPoint{Date: dt, Value: v})
		if at > updated {
			updated = at
		}
	}
	for _, pts := range out {
		sort.Slice(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
	}
	return out, updated, rows.Err()
}

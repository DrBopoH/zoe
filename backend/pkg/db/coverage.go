package db

import (
	"database/sql"
	"sort"
)

// HistoryStart — «начало истории»: интервал покрытия, у которого From == HistoryStart,
// означает, что назад от него снимать больше нечего (банк дальше не отдаёт / там пусто).
const HistoryStart int64 = 0

// Span — снятый (и сохранённый в кеш) отрезок времени счёта, секунды Unix, границы включительно.
type Span struct{ From, To int64 }

// Покрытие живёт в cache.db: сносим кеш — вместе с ним уходит и память о том,
// что «это уже снято», так что следующая синхронизация честно снимет всё заново.
const coverageSchema = `CREATE TABLE IF NOT EXISTS sync_coverage (
	account_id TEXT NOT NULL,
	from_ts INTEGER NOT NULL,
	to_ts   INTEGER NOT NULL,
	PRIMARY KEY (account_id, from_ts)
)`

// Coverage — склеенные отрезки счёта по возрастанию времени.
func (s *Storage) Coverage(accID string) ([]Span, error) {
	rows, err := s.cacheDB.Query(`SELECT from_ts, to_ts FROM sync_coverage WHERE account_id=? ORDER BY from_ts`, accID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Span
	for rows.Next() {
		var sp Span
		if err := rows.Scan(&sp.From, &sp.To); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// AddCoverage записывает снятый отрезок и склеивает его с пересекающимися и соседними.
func (s *Storage) AddCoverage(accID string, from, to int64) error {
	if to < from {
		return nil
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT from_ts, to_ts FROM sync_coverage
		WHERE account_id=? AND from_ts <= ? AND to_ts >= ?`, accID, to+1, from-1)
	if err != nil {
		return err
	}
	var hit []Span
	for rows.Next() {
		var sp Span
		if err := rows.Scan(&sp.From, &sp.To); err != nil {
			rows.Close()
			return err
		}
		hit = append(hit, sp)
	}
	rows.Close()

	merged := Span{from, to}
	for _, sp := range hit {
		if sp.From < merged.From {
			merged.From = sp.From
		}
		if sp.To > merged.To {
			merged.To = sp.To
		}
		if _, err := tx.Exec(`DELETE FROM sync_coverage WHERE account_id=? AND from_ts=?`, accID, sp.From); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO sync_coverage(account_id, from_ts, to_ts) VALUES(?,?,?)`, accID, merged.From, merged.To); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkHistoryComplete — всё от начала времён до `to` считается снятым.
func (s *Storage) MarkHistoryComplete(accID string, to int64) error {
	return s.AddCoverage(accID, HistoryStart, to)
}

// SpanAt — отрезок, содержащий момент t.
func SpanAt(spans []Span, t int64) (Span, bool) {
	for _, sp := range spans {
		if sp.From <= t && t <= sp.To {
			return sp, true
		}
	}
	return Span{}, false
}

// SpanEndingBefore — ближайший по времени отрезок, целиком лежащий ниже t.
func SpanEndingBefore(spans []Span, t int64) (Span, bool) {
	sorted := append([]Span(nil), spans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].To > sorted[j].To })
	for _, sp := range sorted {
		if sp.To < t {
			return sp, true
		}
	}
	return Span{}, false
}

// HasTransactions — есть ли в кеше хоть одна операция счёта в отрезке [from, to] (секунды Unix).
func (s *Storage) HasTransactions(accID string, from, to int64) (bool, error) {
	var one int
	err := s.cacheDB.QueryRow(`SELECT 1 FROM transactions WHERE account_id=? AND time>=? AND time<=? LIMIT 1`, accID, from, to).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// TxTimes — времена всех операций счёта по возрастанию (для таймлайна синка).
func (s *Storage) TxTimes(accID string) ([]int64, error) {
	rows, err := s.cacheDB.Query(`SELECT time FROM transactions WHERE account_id=? ORDER BY time`, accID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var t int64
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

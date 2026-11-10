package db

import (
	"database/sql"
	"time"
)

// Жизненный цикл счёта. Банковские API дат открытия/закрытия не отдают, поэтому берём из выписки:
//   - открытие  = самая старая операция, но только когда история снята целиком (иначе это ещё не дно);
//   - закрытие  = счёт пропал из ответа банка. Дата — последняя операция (если их не было — момент обнаружения).

// OpenedAt — известная дата открытия счёта (0 — неизвестна). Ручная правка (core.db) приоритетнее выписки.
func (s *Storage) OpenedAt(accID string) int64 {
	var v int64
	_ = s.cacheDB.QueryRow(`SELECT COALESCE(opened_at,0) FROM accounts WHERE id=?`, accID).Scan(&v)
	var man sql.NullInt64
	if s.coreDB.QueryRow(`SELECT opened_at FROM account_overrides WHERE account_id=?`, accID).Scan(&man) == nil && man.Valid {
		return man.Int64
	}
	return v
}

// ManualClosedAt — дата закрытия, заданная руками (0 — не задана). Позже неё операций быть не может.
func (s *Storage) ManualClosedAt(accID string) int64 {
	var man sql.NullInt64
	if s.coreDB.QueryRow(`SELECT closed_at FROM account_overrides WHERE account_id=?`, accID).Scan(&man) == nil && man.Valid {
		return man.Int64
	}
	return 0
}

// RecordOpenedAt фиксирует дату открытия по выписке; вызывается, когда обход дошёл до начала истории.
func (s *Storage) RecordOpenedAt(accID string) error {
	var first, cur int64
	_ = s.cacheDB.QueryRow(`SELECT COALESCE(MIN(time),0) FROM transactions WHERE account_id = ?`, accID).Scan(&first)
	_ = s.cacheDB.QueryRow(`SELECT COALESCE(opened_at,0) FROM accounts WHERE id=?`, accID).Scan(&cur)
	if first == 0 || first == cur {
		return nil // нет операций (дата неизвестна) или уже записано — версию данных не дёргаем
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	_, err := s.cacheDB.Exec(`UPDATE accounts SET opened_at = ? WHERE id = ?`, first, accID)
	return err
}

// MarkClosedAbsent помечает закрытыми счета владельца, которых банк больше не отдаёт.
func (s *Storage) MarkClosedAbsent(owner string, presentIDs []string) error {
	if owner == "" || len(presentIDs) == 0 {
		return nil // пустой ответ банка — скорее сбой, чем «закрыл всё»
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	rows, err := s.cacheDB.Query(`SELECT id FROM accounts WHERE source='api' AND owner=? AND COALESCE(closed_at,0)=0`, owner)
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, id := range presentIDs {
		present[id] = true
	}
	var gone []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && !present[id] {
			gone = append(gone, id)
		}
	}
	rows.Close()
	if len(gone) == 0 {
		return nil
	}
	defer s.bump()
	now := time.Now().Unix()
	for _, id := range gone {
		if _, err := s.cacheDB.Exec(`UPDATE accounts SET closed_at = COALESCE(
			NULLIF((SELECT MAX(time) FROM transactions WHERE account_id = ?),0), ?) WHERE id = ?`, id, now, id); err != nil {
			return err
		}
	}
	return nil
}

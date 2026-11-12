package db

import (
	"database/sql"
	"strings"
)

// ───────────────────────── ручные правки счетов ─────────────────────────

type override struct {
	iban, pan      string
	credit         sql.NullFloat64
	opened, closed sql.NullInt64
}

func (s *Storage) loadOverrides() map[string]override {
	out := map[string]override{}
	rows, err := s.coreDB.Query(`SELECT account_id, iban, pan, credit_limit, opened_at, closed_at FROM account_overrides`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var o override
		if rows.Scan(&id, &o.iban, &o.pan, &o.credit, &o.opened, &o.closed) == nil {
			out[id] = o
		}
	}
	return out
}

func (s *Storage) UpdateCreditLimit(accID string, limit float64) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`
		INSERT INTO account_overrides(account_id, credit_limit) VALUES(?, ?)
		ON CONFLICT(account_id) DO UPDATE SET credit_limit = excluded.credit_limit`, accID, limit)
	return err
}

func (s *Storage) UpdateAccountManualDetails(accID, iban, pan string, limit float64) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`
		INSERT INTO account_overrides(account_id, iban, pan, credit_limit) VALUES(?,?,?,?)
		ON CONFLICT(account_id) DO UPDATE SET
			iban = CASE WHEN excluded.iban != '' THEN excluded.iban ELSE account_overrides.iban END,
			pan  = CASE WHEN excluded.pan  != '' THEN excluded.pan  ELSE account_overrides.pan  END,
			credit_limit = excluded.credit_limit`, accID, strings.TrimSpace(iban), strings.TrimSpace(pan), limit)
	return err
}

// ───────────────────────── лица ─────────────────────────

// SetAccountLife — ручные даты открытия/закрытия (unix; 0 — «не задано, считай по выписке»).
// Нужны, когда банк дат не отдаёт. Живут в core.db, поэтому переживают снос кеша.
func (s *Storage) SetAccountLife(accID string, opened, closed int64) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	nul := func(v int64) any {
		if v <= 0 {
			return nil
		}
		return v
	}
	_, err := s.coreDB.Exec(`
		INSERT INTO account_overrides(account_id, opened_at, closed_at) VALUES(?,?,?)
		ON CONFLICT(account_id) DO UPDATE SET opened_at = excluded.opened_at, closed_at = excluded.closed_at`,
		accID, nul(opened), nul(closed))
	return err
}

package db

import "database/sql"

// Связь «карта → IBAN» — факт о самом счёте, а не об алиасе или лице.
// Живёт в core.db (iban_pans), переживает очистку кеша; кеш дополняет её на чтении.

type execer interface {
	Exec(q string, args ...any) (sql.Result, error)
}

// learnIbanPan запоминает глобально: карта pan принадлежит iban.
func learnIbanPan(ex execer, pan, iban string) error {
	pan, iban = trimS(pan), trimS(iban)
	k := idKey(pan)
	if k == "" || iban == "" || !isIBANLike(iban) || isIBANLike(pan) {
		return nil
	}
	_, err := ex.Exec(`
		INSERT INTO iban_pans(pan_key, pan, iban) VALUES(?,?,?)
		ON CONFLICT(pan_key) DO UPDATE SET
			iban = excluded.iban,
			pan  = CASE WHEN instr(excluded.pan,'*')=0 THEN excluded.pan ELSE iban_pans.pan END`,
		k, pan, normID(iban))
	return err
}

func trimS(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// globalPans — normID(IBAN) → карты: сохранённые в iban_pans ∪ известные кешу.
func (s *Storage) globalPans(idx *cacheIndex) map[string][]string {
	out := map[string][]string{}
	seen := map[string]bool{}
	add := func(iban, pan string) {
		k := idKey(pan)
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		out[iban] = append(out[iban], pan)
	}
	if idx != nil {
		for iban, pans := range idx.pansByIBAN {
			for _, p := range pans {
				add(iban, p)
			}
		}
	}
	rows, err := s.coreDB.Query(`SELECT pan, iban FROM iban_pans`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var pan, iban string
		if rows.Scan(&pan, &iban) == nil {
			add(normID(iban), pan)
		}
	}
	return out
}

// backfillIbanPans переносит старые per-alias связи в глобальную таблицу (один раз, идемпотентно).
func (s *Storage) backfillIbanPans() {
	rows, err := s.coreDB.Query(`SELECT account_identifier, parent_identifier FROM alias_accounts WHERE parent_identifier != ''`)
	if err != nil {
		return
	}
	type pr struct{ pan, iban string }
	var list []pr
	for rows.Next() {
		var p pr
		if rows.Scan(&p.pan, &p.iban) == nil {
			list = append(list, p)
		}
	}
	rows.Close()
	for _, p := range list {
		_ = learnIbanPan(s.coreDB, p.pan, p.iban)
	}
}

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
)

// Алиасы лиц: запомненные структуры, привязка и отвязка, синхронизация с кешем.

// GetAliasStructures — запомненные структуры всех алиасов, включая бесхозные.
func (s *Storage) GetAliasStructures() ([]AliasStructure, error) {
	idx, _ := s.loadCacheIndex()
	rows, err := s.coreDB.Query(`
		SELECT aa.alias_key, aa.alias, aa.account_identifier, aa.parent_identifier, COALESCE(ua.user_id,'')
		FROM alias_accounts aa LEFT JOIN user_aliases ua ON ua.alias_key = aa.alias_key
		ORDER BY aa.alias_key, aa.account_identifier`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AliasStructure{}
	pos := map[string]int{}
	global := s.globalPans(idx)
	for rows.Next() {
		var k, alias, id, parent, uid string
		if rows.Scan(&k, &alias, &id, &parent, &uid) != nil {
			continue
		}
		i, ok := pos[k]
		if !ok {
			i = len(out)
			pos[k] = i
			out = append(out, AliasStructure{Alias: alias, UserID: uid, Accounts: []AliasLink{}})
		}
		if parent == "" && !isIBANLike(id) {
			parent = idx.parentFor(id)
		}
		out[i].Accounts = append(out[i].Accounts, AliasLink{alias, id, parent, idx.confirms(k, id)})
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows.Close()

	// IBAN в алиасе тянет за собой все свои карты (глобальный факт), кроме снятых вручную (баны)
	bans := map[string]map[string]bool{}
	if br, err := s.coreDB.Query(`SELECT alias_key, account_identifier FROM alias_bans`); err == nil {
		for br.Next() {
			var k, id string
			if br.Scan(&k, &id) == nil {
				if bans[k] == nil {
					bans[k] = map[string]bool{}
				}
				bans[k][idKey(id)] = true
			}
		}
		br.Close()
	}
	for k, i := range pos {
		have := map[string]bool{}
		for _, l := range out[i].Accounts {
			have[idKey(l.AccountIdentifier)] = true
		}
		cur := out[i].Accounts
		for _, l := range cur {
			if !isIBANLike(l.AccountIdentifier) {
				continue
			}
			for _, p := range global[normID(l.AccountIdentifier)] {
				pk := idKey(p)
				if have[pk] || bans[k][pk] {
					continue
				}
				have[pk] = true
				out[i].Accounts = append(out[i].Accounts, AliasLink{out[i].Alias, p, l.AccountIdentifier, idx.confirms(k, p)})
			}
		}
	}
	return out, nil
}

// ───────────────────────── алиасы ─────────────────────────

// attachTx привязывает алиас к лицу. При смене владельца баны гаснут.
func attachTx(tx *sql.Tx, userID, alias string) error {
	k := aliasKey(alias)
	if k == "" {
		return errors.New("empty alias")
	}
	var prev string
	err := tx.QueryRow(`SELECT user_id FROM user_aliases WHERE alias_key=?`, k).Scan(&prev)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil && prev != userID {
		if _, err := tx.Exec(`DELETE FROM alias_bans WHERE alias_key=?`, k); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`
		INSERT INTO user_aliases(alias_key, alias, user_id) VALUES(?,?,?)
		ON CONFLICT(alias_key) DO UPDATE SET user_id = excluded.user_id`, k, strings.TrimSpace(alias), userID)
	return err
}

// AddUserAlias: «добавить алиас к лицу» — момент, когда структура из кеша
// записывается в статику.
func (s *Storage) AddUserAlias(userID, alias string) error {
	if userID == "" {
		return errors.New("empty user")
	}
	defer s.bump()
	s.coreMu.Lock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		s.coreMu.Unlock()
		return err
	}
	err = attachTx(tx, userID, alias)
	if err == nil {
		err = tx.Commit()
	} else {
		tx.Rollback()
	}
	s.coreMu.Unlock()
	if err != nil {
		return err
	}
	s.syncAliases([]string{aliasKey(alias)})
	return nil
}

// RemoveUserAlias: отвязка алиаса от лица. Структура остаётся в статике
// (алиас «помнит» её в бесхозе), теневые баны гаснут.
func (s *Storage) RemoveUserAlias(alias string) error {
	defer s.bump()
	k := aliasKey(alias)
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM user_aliases WHERE alias_key=?`, k); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM alias_bans WHERE alias_key=?`, k); err != nil {
		return err
	}
	return tx.Commit()
}

// ───────────────────────── связи алиас ↔ счёт ─────────────────────────

// syncAliases — ЕДИНСТВЕННЫЙ путь данных из кеша в статику. Работает только для
// алиасов, привязанных к лицу, подтягивает только то, что подтверждает кеш,
// и никогда не воскрешает теневые баны. only == nil → все привязанные алиасы.
func (s *Storage) syncAliases(only []string) {
	idx, err := s.loadCacheIndex()
	if err != nil {
		log.Printf("[DB] sync: cache index: %v", err)
		return
	}
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	if err := s.syncAliasesLocked(idx, only); err != nil {
		log.Printf("[DB] sync aliases: %v", err)
	}
	s.bump()
}

func (s *Storage) syncAliasesLocked(idx *cacheIndex, only []string) error {
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT alias_key, alias FROM user_aliases`)
	if err != nil {
		return err
	}
	attached := map[string]string{}
	for rows.Next() {
		var k, a string
		if rows.Scan(&k, &a) == nil {
			attached[k] = a
		}
	}
	rows.Close()

	want := map[string]bool{}
	for _, k := range only {
		want[k] = true
	}

	for k, display := range attached {
		if only != nil && !want[k] {
			continue
		}
		links := idx.byAlias[k]
		if len(links) == 0 {
			continue
		}

		type have struct{ id, parent string }
		var existing []have
		r1, err := tx.Query(`SELECT account_identifier, parent_identifier FROM alias_accounts WHERE alias_key=?`, k)
		if err != nil {
			return err
		}
		for r1.Next() {
			var h have
			if r1.Scan(&h.id, &h.parent) == nil {
				existing = append(existing, h)
			}
		}
		r1.Close()

		bans := map[string]bool{}
		r2, err := tx.Query(`SELECT account_identifier FROM alias_bans WHERE alias_key=?`, k)
		if err != nil {
			return err
		}
		for r2.Next() {
			var id string
			if r2.Scan(&id) == nil {
				bans[idKey(id)] = true
			}
		}
		r2.Close()

		for _, l := range links {
			ik := idKey(l.ID)
			if bans[ik] || (l.Parent != "" && bans[idKey(l.Parent)]) {
				continue
			}
			found := -1
			for i, h := range existing {
				if sameID(h.id, l.ID) {
					found = i
					break
				}
			}
			if found >= 0 {
				h := existing[found]
				if isPartialPAN(h.id) && !isPartialPAN(l.ID) {
					// в статике была частичная карта, кеш принёс полную — это та же карта, заменяем
					if _, err := tx.Exec(`UPDATE alias_accounts SET account_identifier=?, parent_identifier=CASE WHEN ?!='' THEN ? ELSE parent_identifier END WHERE alias_key=? AND account_identifier=?`,
						l.ID, l.Parent, l.Parent, k, h.id); err != nil {
						return err
					}
					existing[found] = have{l.ID, firstNonEmpty(l.Parent, h.parent)}
				} else if l.Parent != "" && h.parent != l.Parent { // кеш — источник истины
					if _, err := tx.Exec(`UPDATE alias_accounts SET parent_identifier=? WHERE alias_key=? AND account_identifier=?`, l.Parent, k, h.id); err != nil {
						return err
					}
					existing[found].parent = l.Parent
				}
				continue
			}
			if _, err := tx.Exec(`INSERT INTO alias_accounts(alias_key, alias, account_identifier, parent_identifier) VALUES(?,?,?,?)`,
				k, display, l.ID, l.Parent); err != nil {
				return fmt.Errorf("insert %s/%s: %w", k, l.ID, err)
			}
			existing = append(existing, have{l.ID, l.Parent})
		}
	}
	return tx.Commit()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

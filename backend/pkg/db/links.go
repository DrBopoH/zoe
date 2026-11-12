package db

import (
	"database/sql"
	"errors"
	"strings"
)

// Связи счетов с лицами: ручное добавление, снятие и «теневые» баны.

// LinkAccount — ручное добавление связи (модалка / «добавить к лицу»).
// Кеш — источник истины: если он знает, к какому IBAN относится PAN, берём его.
// Явное добавление снимает теневой бан на эту связь.
func (s *Storage) LinkAccount(userID, alias, identifier, parent string) error {
	defer s.bump()
	k := aliasKey(alias)
	identifier = strings.TrimSpace(identifier)
	if k == "" || identifier == "" {
		return errors.New("alias and account_identifier are required")
	}
	idx, _ := s.loadCacheIndex()
	parent = strings.TrimSpace(parent)
	if isIBANLike(identifier) {
		parent = ""
	} else if p := idx.parentFor(identifier); p != "" {
		parent = p
	}

	global := s.globalPans(idx) // до транзакции: читает core.db
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	display := strings.TrimSpace(alias)
	if userID != "" {
		if err := attachTx(tx, userID, alias); err != nil {
			return err
		}
	}
	var cur string
	if err := tx.QueryRow(`SELECT alias FROM user_aliases WHERE alias_key=?`, k).Scan(&cur); err == nil {
		display = cur
	}

	// уже есть эта связь под другим написанием маски? — обновляем её, а не плодим дубль
	rows, err := tx.Query(`SELECT account_identifier FROM alias_accounts WHERE alias_key=?`, k)
	if err != nil {
		return err
	}
	var existing []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			existing = append(existing, id)
		}
	}
	rows.Close()
	target := identifier
	for _, id := range existing {
		if idKey(id) != idKey(identifier) {
			continue
		}
		target = id // то же самое написание — не плодим дубль
		if isMaskedID(id) && !isMaskedID(identifier) {
			// в базе была маска, пришёл настоящий номер с теми же концами:
			// это та же карта (остаётся на своём месте и остаётся главной), но в базе теперь чистое число
			if _, err := tx.Exec(`UPDATE alias_accounts SET account_identifier=? WHERE alias_key=? AND account_identifier=?`, identifier, k, id); err != nil {
				return err
			}
			target = identifier
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO alias_accounts(alias_key, alias, account_identifier, parent_identifier) VALUES(?,?,?,?)
		ON CONFLICT(alias_key, account_identifier) DO UPDATE SET
			alias = excluded.alias,
			parent_identifier = CASE WHEN excluded.parent_identifier != '' THEN excluded.parent_identifier ELSE alias_accounts.parent_identifier END`,
		k, display, target, parent); err != nil {
		return err
	}
	if err := deleteBansTx(tx, k, identifier); err != nil {
		return err
	}
	// добавили IBAN вручную — его карты снова наши: снимаем и их баны (UnlinkAccount банит карты каскадом вместе с IBAN)
	if isIBANLike(identifier) {
		for _, p := range global[normID(identifier)] {
			if err := deleteBansTx(tx, k, p); err != nil {
				return err
			}
		}
	}
	// глобальный факт: карта → IBAN (для всех алиасов сразу)
	if parent != "" && !isIBANLike(identifier) {
		if err := learnIbanPan(tx, identifier, parent); err != nil {
			return err
		}
	}
	// добавили IBAN — сохраняем его известные кешу карты, чтобы они пережили очистку кеша
	if isIBANLike(identifier) {
		for _, p := range idx.pansByIBAN[normID(identifier)] {
			if err := learnIbanPan(tx, p, identifier); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func deleteBansTx(tx *sql.Tx, k, identifier string) error {
	rows, err := tx.Query(`SELECT account_identifier FROM alias_bans WHERE alias_key=?`, k)
	if err != nil {
		return err
	}
	var kill []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && idKey(id) == idKey(identifier) {
			kill = append(kill, id)
		}
	}
	rows.Close()
	for _, id := range kill {
		if _, err := tx.Exec(`DELETE FROM alias_bans WHERE alias_key=? AND account_identifier=?`, k, id); err != nil {
			return err
		}
	}
	return nil
}

// UnlinkAccount — убрать счёт/карту из структуры алиаса.
//
//	IBAN  → уходит и сам IBAN, и все его PAN в этом алиасе
//	PAN   → уходит только карта
//
// Если алиас привязан к лицу и ту же связь подтверждает кеш — вешаем теневой бан,
// иначе кеш тут же «воскресил» бы удалённое. У бесхозного алиаса бана нет:
// кеш и так показывает свою структуру в рантайме.
// alias == "" — убрать из всех алиасов, где идентификатор встречается.
func (s *Storage) UnlinkAccount(alias, identifier string) error {
	defer s.bump()
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return errors.New("identifier is required")
	}
	idx, _ := s.loadCacheIndex()

	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var keys []string
	if k := aliasKey(alias); k != "" {
		keys = []string{k}
	} else {
		rows, err := tx.Query(`SELECT alias_key FROM user_aliases UNION SELECT alias_key FROM alias_accounts`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var k string
			if rows.Scan(&k) == nil {
				keys = append(keys, k)
			}
		}
		rows.Close()
	}

	global := s.globalPans(idx)
	target := idKey(identifier)
	isIBAN := isIBANLike(identifier)
	hit := func(id, parent string) bool {
		return idKey(id) == target || (isIBAN && parent != "" && idKey(parent) == target)
	}

	for _, k := range keys {
		var tmp string
		attached := tx.QueryRow(`SELECT 1 FROM user_aliases WHERE alias_key=?`, k).Scan(&tmp) == nil

		rows, err := tx.Query(`SELECT account_identifier, parent_identifier FROM alias_accounts WHERE alias_key=?`, k)
		if err != nil {
			return err
		}
		type row struct{ id, parent string }
		var kill []row
		for rows.Next() {
			var r row
			if rows.Scan(&r.id, &r.parent) == nil && hit(r.id, r.parent) {
				kill = append(kill, r)
			}
		}
		rows.Close()

		banned := map[string]bool{}
		ban := func(id string) error {
			if banned[idKey(id)] {
				return nil
			}
			banned[idKey(id)] = true
			_, err := tx.Exec(`INSERT OR IGNORE INTO alias_bans(alias_key, account_identifier) VALUES(?,?)`, k, id)
			return err
		}

		for _, r := range kill {
			if _, err := tx.Exec(`DELETE FROM alias_accounts WHERE alias_key=? AND account_identifier=?`, k, r.id); err != nil {
				return err
			}
			derived := !isIBANLike(r.id) && r.parent != "" && globalHas(global, r.parent, r.id)
			if (attached && idx.confirms(k, r.id)) || derived {
				if err := ban(r.id); err != nil {
					return err
				}
			}
		}
		// карта, которую алиас получает глобально через свой IBAN (своей строки нет) — тоже под бан
		if !isIBAN {
			var ibans []string
			for _, ex := range existingIDs(tx, k) {
				if isIBANLike(ex) && globalHas(global, ex, identifier) {
					ibans = append(ibans, ex)
				}
			}
			if len(ibans) > 0 {
				if err := ban(identifier); err != nil {
					return err
				}
			}
		}
		// связи, которые есть только в кеше (в статику ещё не доехали) — тоже под бан
		if attached && idx != nil {
			for _, l := range idx.byAlias[k] {
				if hit(l.ID, l.Parent) {
					if err := ban(l.ID); err != nil {
						return err
					}
				}
			}
		}
	}
	return tx.Commit()
}

// ───────────────────────── кеш → статика ─────────────────────────

func globalHas(g map[string][]string, iban, pan string) bool {
	for _, p := range g[normID(iban)] {
		if idKey(p) == idKey(pan) {
			return true
		}
	}
	return false
}

func existingIDs(tx *sql.Tx, k string) []string {
	rows, err := tx.Query(`SELECT account_identifier FROM alias_accounts WHERE alias_key=?`, k)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out
}

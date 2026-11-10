package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
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

func (s *Storage) CreateUser(u User) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`INSERT INTO users(id, name, role, avatar_url) VALUES(?,?,?,?)`, u.ID, u.Name, orDefault(u.Role, "DONOR"), u.AvatarURL)
	return err
}

func (s *Storage) UpdateUser(u User) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	if u.Role == "" {
		_, err := s.coreDB.Exec(`UPDATE users SET name=?, avatar_url=? WHERE id=?`, u.Name, u.AvatarURL, u.ID)
		return err
	}
	_, err := s.coreDB.Exec(`UPDATE users SET name=?, role=?, avatar_url=? WHERE id=?`, u.Name, u.Role, u.AvatarURL, u.ID)
	return err
}

// DeleteUser: алиасы отвязываются (каскад), баны гаснут вместе с привязкой,
// запомненная структура алиасов остаётся — алиасы просто становятся бесхозными.
func (s *Storage) DeleteUser(id string) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM alias_bans WHERE alias_key IN (SELECT alias_key FROM user_aliases WHERE user_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_aliases WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM users WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// GetUsers собирает лиц целиком из статики; зелёный/жёлтый статус (Confirmed)
// считается по текущему кешу.
func (s *Storage) GetUsers() ([]User, error) {
	idx, _ := s.loadCacheIndex()

	urows, err := s.coreDB.Query(`SELECT id, name, role, COALESCE(avatar_url,'') FROM users ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	users := []User{}
	for urows.Next() {
		var u User
		if err := urows.Scan(&u.ID, &u.Name, &u.Role, &u.AvatarURL); err != nil {
			urows.Close()
			return nil, err
		}
		u.Aliases, u.AliasLinks, u.AliasBans = []string{}, []AliasLink{}, []AliasLink{}
		u.Accounts, u.IBANs = []UserAccount{}, []string{}
		users = append(users, u)
	}
	urows.Close()

	userIdx := map[string]int{}
	for i, u := range users {
		userIdx[u.ID] = i
	}
	type att struct {
		uid, alias string
	}
	attached := map[string]att{} // alias_key → лицо/название

	arows, err := s.coreDB.Query(`SELECT alias_key, alias, user_id FROM user_aliases ORDER BY alias_key`)
	if err != nil {
		return nil, err
	}
	for arows.Next() {
		var k, a, uid string
		if arows.Scan(&k, &a, &uid) == nil {
			if i, ok := userIdx[uid]; ok {
				users[i].Aliases = append(users[i].Aliases, a)
				attached[k] = att{uid, a}
			}
		}
	}
	arows.Close()

	lrows, err := s.coreDB.Query(`SELECT alias_key, account_identifier, parent_identifier FROM alias_accounts ORDER BY alias_key, account_identifier`)
	if err != nil {
		return nil, err
	}
	type lk struct{ k, id, parent string }
	var links []lk
	for lrows.Next() {
		var l lk
		if lrows.Scan(&l.k, &l.id, &l.parent) == nil {
			links = append(links, l)
		}
	}
	lrows.Close()

	accPos := map[string]map[string]int{} // user → idKey → позиция в Accounts
	for _, l := range links {
		a, ok := attached[l.k]
		if !ok {
			continue
		}
		u := &users[userIdx[a.uid]]
		conf := idx.confirms(l.k, l.id)
		u.AliasLinks = append(u.AliasLinks, AliasLink{a.alias, l.id, l.parent, conf})

		if accPos[a.uid] == nil {
			accPos[a.uid] = map[string]int{}
		}
		k := idKey(l.id)
		if pos, dup := accPos[a.uid][k]; dup {
			ua := &u.Accounts[pos]
			if ua.ParentIdentifier == "" {
				ua.ParentIdentifier = l.parent
			}
			ua.Confirmed = ua.Confirmed || conf
			continue
		}
		accPos[a.uid][k] = len(u.Accounts)
		u.Accounts = append(u.Accounts, UserAccount{l.id, l.parent, a.alias, conf})
		u.IBANs = append(u.IBANs, l.id)
	}

	brows, err := s.coreDB.Query(`SELECT alias_key, account_identifier FROM alias_bans ORDER BY alias_key, account_identifier`)
	if err != nil {
		return nil, err
	}
	for brows.Next() {
		var k, id string
		if brows.Scan(&k, &id) == nil {
			if a, ok := attached[k]; ok {
				u := &users[userIdx[a.uid]]
				u.AliasBans = append(u.AliasBans, AliasLink{Alias: a.alias, AccountIdentifier: id})
			}
		}
	}
	brows.Close()
	return users, nil
}

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

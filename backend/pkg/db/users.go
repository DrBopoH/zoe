package db

// Лица (Core DB): создание, правка, удаление и чтение со всеми алиасами и счетами.

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

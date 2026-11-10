package db

import "log"

// ───────────── индекс «что кеш знает про алиасы» ─────────────

type cacheLink struct{ ID, Parent string }

type cacheIndex struct {
	byAlias    map[string][]cacheLink // alias_key → структура, подтверждённая кешем
	ibanByPAN  map[string]string      // idKey(PAN) → IBAN счёта-владельца
	pansByIBAN map[string][]string    // normID(IBAN) → PAN счёта (как в кеше)
}

// confirms — подтверждает ли кеш связь «алиас → идентификатор» прямо сейчас.
func (c *cacheIndex) confirms(aliasK, id string) bool {
	if c == nil {
		return false
	}
	for _, l := range c.byAlias[aliasK] {
		if sameID(l.ID, id) {
			return true
		}
	}
	return false
}

// parentFor — IBAN, к которому кеш относит этот PAN ("" если неизвестно).
func (c *cacheIndex) parentFor(pan string) string {
	if c == nil {
		return ""
	}
	return c.ibanByPAN[idKey(pan)]
}

// loadCacheIndex строит индекс из кеша. Правила совпадают с фронтом (buildCacheIndex):
//  1. имя аккаунта в банке (accounts.name без «(тип)») → его IBAN и все PAN
//  2. контрагент операции (counter_name, а если пусто — контакт из description)
//     → counter_iban (+ PAN этого счёта, если он нам известен)
//
// Сравнение — точное, без LIKE.
func (s *Storage) loadCacheIndex() (*cacheIndex, error) {
	accs, err := s.GetAccounts()
	if err != nil {
		return nil, err
	}
	idx := &cacheIndex{byAlias: map[string][]cacheLink{}, ibanByPAN: map[string]string{}, pansByIBAN: map[string][]string{}}
	add := func(key string, l cacheLink) {
		if key == "" || l.ID == "" {
			return
		}
		list := idx.byAlias[key]
		for i, ex := range list {
			if !sameID(ex.ID, l.ID) {
				continue
			}
			// та же карта: полный номер вытесняет частичный (414949****72 → 414949******0172)
			if isPartialPAN(ex.ID) && !isPartialPAN(l.ID) {
				list[i].ID = l.ID
				if l.Parent != "" {
					list[i].Parent = l.Parent
				}
			} else if ex.Parent == "" && l.Parent != "" {
				list[i].Parent = l.Parent
			}
			return
		}
		idx.byAlias[key] = append(list, l)
	}

	byIBAN := map[string]Account{}
	byPAN := map[string]Account{}     // idKey(PAN) → наш счёт
	byLoose := map[string][]Account{} // BIN6+последние2 → наши счета (используем, только если он один)
	for _, a := range accs {
		if a.IBAN != "" {
			byIBAN[normID(a.IBAN)] = a
		}
		for _, p := range a.Pans {
			byPAN[idKey(p)] = a
			if lk := looseKey(p); lk != "" {
				byLoose[lk] = append(byLoose[lk], a)
			}
		}
		for _, p := range a.Pans {
			if a.IBAN != "" {
				idx.ibanByPAN[idKey(p)] = a.IBAN
				idx.pansByIBAN[normID(a.IBAN)] = append(idx.pansByIBAN[normID(a.IBAN)], p)
			}
		}
		key := aliasKey(cleanAccountName(a.Name))
		if a.IBAN != "" {
			add(key, cacheLink{a.IBAN, ""})
		}
		for _, p := range a.Pans {
			add(key, cacheLink{p, a.IBAN})
		}
	}

	// ownByPAN: чей это счёт по номеру карты контрагента (точно по idKey, иначе по BIN+2 при однозначности)
	ownByPAN := func(pan string) (Account, bool) {
		if k := reliableKey(pan); k != "" {
			if a, ok := byPAN[k]; ok {
				return a, true
			}
		}
		if lk := looseKey(pan); lk != "" && len(byLoose[lk]) == 1 {
			return byLoose[lk][0], true
		}
		return Account{}, false
	}

	rows, err := s.cacheDB.Query(`
		SELECT DISTINCT counter_iban,
		       CASE WHEN COALESCE(counter_name,'') != '' THEN counter_name ELSE '' END,
		       CASE WHEN COALESCE(counter_name,'')  = '' THEN COALESCE(description,'') ELSE '' END
		FROM transactions WHERE COALESCE(counter_iban,'') != ''`)
	if err != nil {
		return idx, nil // без операций индекс всё равно полезен
	}
	defer rows.Close()
	for rows.Next() {
		var iban, cname, desc string
		if rows.Scan(&iban, &cname, &desc) != nil {
			continue
		}
		key := aliasKey(cleanPersonName(cname))
		if key == "" {
			key = aliasKey(cleanPersonName(desc))
		}
		if key == "" {
			key = aliasKey(pseudoCounterName(iban, desc))
		}
		if isPanLike(iban) {
			// контрагент указан номером карты (парсер ПриватБанка): если карта наша — тянем весь счёт
			if a, ok := ownByPAN(iban); ok && a.IBAN != "" {
				add(key, cacheLink{a.IBAN, ""})
				for _, p := range a.Pans {
					add(key, cacheLink{p, a.IBAN})
				}
			} else {
				add(key, cacheLink{iban, ""})
			}
			continue
		}
		add(key, cacheLink{iban, ""})
		if a, ok := byIBAN[normID(iban)]; ok {
			for _, p := range a.Pans {
				add(key, cacheLink{p, a.IBAN})
			}
		}
	}
	return idx, nil
}

// ───────────── резолв операций → лицо (на чтении, кеш не пишем) ─────────────

type resolver struct {
	byID    map[string]string // idKey → user_id
	byLoose map[string]string // looseKey (BIN6+2) → user_id, только однозначные
	byAlias map[string]string // alias_key → user_id
}

func (s *Storage) buildResolver() *resolver {
	idx, _ := s.loadCacheIndex()
	global := s.globalPans(idx)
	r := &resolver{byID: map[string]string{}, byLoose: map[string]string{}, byAlias: map[string]string{}}
	looseOwner := map[string]string{}
	looseAmbiguous := map[string]bool{}
	rows, err := s.coreDB.Query(`
		SELECT ua.user_id, ua.alias_key, COALESCE(aa.account_identifier,'')
		FROM user_aliases ua LEFT JOIN alias_accounts aa ON aa.alias_key = ua.alias_key`)
	if err != nil {
		log.Printf("[DB] resolver: %v", err)
		return r
	}
	defer rows.Close()
	for rows.Next() {
		var uid, ak, id string
		if rows.Scan(&uid, &ak, &id) != nil {
			continue
		}
		r.byAlias[ak] = uid
		if id != "" && isIBANLike(id) {
			for _, p := range global[normID(id)] {
				if _, taken := r.byID[idKey(p)]; !taken {
					r.byID[idKey(p)] = uid
				}
			}
		}
		if id != "" {
			r.byID[idKey(id)] = uid
			if lk := looseKey(id); lk != "" {
				if prev, ok := looseOwner[lk]; ok && prev != uid {
					looseAmbiguous[lk] = true
				}
				looseOwner[lk] = uid
			}
		}
	}
	for lk, uid := range looseOwner {
		if !looseAmbiguous[lk] {
			r.byLoose[lk] = uid
		}
	}
	return r
}

func (r *resolver) resolve(counterIBAN, counterName, description string) string {
	if k := reliableKey(counterIBAN); k != "" {
		if u, ok := r.byID[k]; ok {
			return u
		}
	}
	if lk := looseKey(counterIBAN); lk != "" {
		if u, ok := r.byLoose[lk]; ok {
			return u
		}
	}
	key := aliasKey(cleanPersonName(counterName))
	if key == "" {
		key = aliasKey(cleanPersonName(description))
	}
	if key == "" {
		key = aliasKey(pseudoCounterName(counterIBAN, description))
	}
	return r.byAlias[key] // "" если не найдено
}

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

// add добавляет связь «алиас → идентификатор» в индекс, сливая дубли.
func (c *cacheIndex) add(key string, l cacheLink) {
	if key == "" || l.ID == "" {
		return
	}
	list := c.byAlias[key]
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
	c.byAlias[key] = append(list, l)
}

// addAccount добавляет под ключом алиаса весь счёт: его IBAN и каждую карту (с IBAN как родителем).
func (c *cacheIndex) addAccount(key string, a Account) {
	if a.IBAN != "" {
		c.add(key, cacheLink{a.IBAN, ""})
	}
	for _, p := range a.Pans {
		c.add(key, cacheLink{p, a.IBAN})
	}
}

// ownAccounts — наши счета, найденные по IBAN, по карте и по «ослабленному» номеру карты.
type ownAccounts struct {
	byIBAN  map[string]Account
	byPAN   map[string]Account   // idKey(PAN) → наш счёт
	byLoose map[string][]Account // BIN6+последние2 → наши счета (используем, только если он один)
}

// indexAccounts строит индекс по нашим счетам: имя аккаунта в банке даёт его IBAN и все карты; заодно заполняет связи карта↔IBAN.
func indexAccounts(idx *cacheIndex, accs []Account) ownAccounts {
	own := ownAccounts{byIBAN: map[string]Account{}, byPAN: map[string]Account{}, byLoose: map[string][]Account{}}
	for _, a := range accs {
		if a.IBAN != "" {
			own.byIBAN[normID(a.IBAN)] = a
		}
		for _, p := range a.Pans {
			own.byPAN[idKey(p)] = a
			if lk := looseKey(p); lk != "" {
				own.byLoose[lk] = append(own.byLoose[lk], a)
			}
		}
		for _, p := range a.Pans {
			if a.IBAN != "" {
				idx.ibanByPAN[idKey(p)] = a.IBAN
				idx.pansByIBAN[normID(a.IBAN)] = append(idx.pansByIBAN[normID(a.IBAN)], p)
			}
		}
		idx.addAccount(aliasKey(cleanAccountName(a.Name)), a)
	}
	return own
}

// byCard — чей это счёт по номеру карты контрагента: точно по idKey, иначе по BIN+2 при однозначности.
func (o ownAccounts) byCard(pan string) (Account, bool) {
	if k := reliableKey(pan); k != "" {
		if a, ok := o.byPAN[k]; ok {
			return a, true
		}
	}
	if lk := looseKey(pan); lk != "" && len(o.byLoose[lk]) == 1 {
		return o.byLoose[lk][0], true
	}
	return Account{}, false
}

// indexCounterparties добавляет в индекс контрагентов операций: имя (или контакт из описания) → counter_iban и, если он наш, весь счёт.
func (s *Storage) indexCounterparties(idx *cacheIndex, own ownAccounts) {
	rows, err := s.cacheDB.Query(`
		SELECT DISTINCT counter_iban,
		       CASE WHEN COALESCE(counter_name,'') != '' THEN counter_name ELSE '' END,
		       CASE WHEN COALESCE(counter_name,'')  = '' THEN COALESCE(description,'') ELSE '' END
		FROM transactions WHERE COALESCE(counter_iban,'') != ''`)
	if err != nil {
		return // без операций индекс всё равно полезен
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
			if a, ok := own.byCard(iban); ok && a.IBAN != "" {
				idx.addAccount(key, a)
			} else {
				idx.add(key, cacheLink{iban, ""})
			}
			continue
		}
		idx.add(key, cacheLink{iban, ""})
		if a, ok := own.byIBAN[normID(iban)]; ok {
			for _, p := range a.Pans {
				idx.add(key, cacheLink{p, a.IBAN})
			}
		}
	}
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
	own := indexAccounts(idx, s.withBinanceProfile(accs))
	s.indexCounterparties(idx, own)
	return idx, nil
}

// ───────────── резолв операций → лицо (на чтении, кеш не пишем) ─────────────

type resolver struct {
	byID    map[string]string      // idKey → user_id
	byLoose map[string]string      // looseKey (BIN6+2) → user_id, только однозначные
	byAlias map[string]string      // alias_key → user_id
	accts   map[string][]AliasAcct // alias_key → счета/карты алиаса (core.db + то, что подтвердил кеш)
}

// AliasAcct — счёт или карта, привязанные к сырому алиасу контрагента. Confirmed: кеш подтверждает связь прямо сейчас.
type AliasAcct struct {
	ID        string `json:"id"`
	Confirmed bool   `json:"confirmed,omitempty"`
}

func (s *Storage) buildResolver() *resolver {
	idx, _ := s.loadCacheIndex()
	global := s.globalPans(idx)
	r := &resolver{byID: map[string]string{}, byLoose: map[string]string{}, byAlias: map[string]string{}, accts: s.aliasAccts(idx)}
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
	return r.byAlias[txAliasKey(counterIBAN, counterName, description)] // "" если не найдено
}

// txAliasKey — алиас контрагента операции: имя, иначе контакт из описания, иначе псевдо-алиас по карте.
func txAliasKey(counterIBAN, counterName, description string) string {
	key := aliasKey(cleanPersonName(counterName))
	if key == "" {
		key = aliasKey(cleanPersonName(description))
	}
	if key == "" {
		key = aliasKey(pseudoCounterName(counterIBAN, description))
	}
	return key
}

// aliasAccts собирает по каждому алиасу его счета и карты: сначала то, что привязано в core.db, затем то, что знает кеш.
// «Листья»: IBAN, у которого в списке есть своя карта, не дублируем (для чипа нужна именно карта).
func (s *Storage) aliasAccts(idx *cacheIndex) map[string][]AliasAcct {
	raw := map[string][]cacheLink{}
	add := func(k string, l cacheLink) {
		if k == "" || l.ID == "" {
			return
		}
		for _, ex := range raw[k] {
			if sameID(ex.ID, l.ID) {
				return
			}
		}
		raw[k] = append(raw[k], l)
	}
	if rows, err := s.coreDB.Query(`SELECT alias_key, account_identifier, COALESCE(parent_identifier,'') FROM alias_accounts ORDER BY alias_key, account_identifier`); err == nil {
		for rows.Next() {
			var k, id, parent string
			if rows.Scan(&k, &id, &parent) == nil {
				add(k, cacheLink{id, parent})
			}
		}
		rows.Close()
	}
	if idx != nil {
		for k, links := range idx.byAlias {
			for _, l := range links {
				add(k, l)
			}
		}
	}
	out := make(map[string][]AliasAcct, len(raw))
	for k, links := range raw {
		hasChild := map[string]bool{}
		for _, l := range links {
			if l.Parent != "" {
				hasChild[normID(l.Parent)] = true
			}
		}
		var confirmed, rest []AliasAcct
		for _, l := range links {
			if isIBANLike(l.ID) && hasChild[normID(l.ID)] {
				continue
			}
			a := AliasAcct{ID: l.ID, Confirmed: idx.confirms(k, l.ID)}
			if a.Confirmed {
				confirmed = append(confirmed, a)
			} else {
				rest = append(rest, a)
			}
		}
		if list := append(confirmed, rest...); len(list) > 0 {
			out[k] = list
		}
	}
	return out
}

// aliasAccts — счета и карты сырого алиаса операции (того, что подменяется Лицом).
func (r *resolver) aliasAccts(counterIBAN, counterName, description string) []AliasAcct {
	return r.accts[txAliasKey(counterIBAN, counterName, description)]
}

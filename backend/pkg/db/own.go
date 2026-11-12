package db

import "regexp"

// ───────────── «это мой счёт»: контрагент операции — другой наш счёт или карта ─────────────
// Номер контрагента берётся из counter_iban и из текста операции (имя/описание: «Грошові перекази: 5175 47** **** 0170»).
// Счёт самой операции не считается: в описании зачисления своя карта стоит рядом с картой отправителя.

var reCardInText = regexp.MustCompile(`(\d{4})[\d ]*[*•xX][*•xX ]*(\d{4})`)

type ownMatcher struct {
	byIBAN  map[string]string   // normID(IBAN) → account id
	byPAN   map[string]string   // idKey(PAN) → account id
	byLoose map[string][]string // looseKey(PAN) → account ids (берём, только если счёт один)
	ids     map[string][]string // account id → его IBAN и карты (чтобы найти Лицо-владельца)
}

func (s *Storage) buildOwnMatcher() *ownMatcher {
	m := &ownMatcher{byIBAN: map[string]string{}, byPAN: map[string]string{}, byLoose: map[string][]string{}, ids: map[string][]string{}}
	accs, err := s.GetAccounts()
	if err != nil {
		return m
	}
	for _, a := range accs {
		if a.IBAN != "" {
			m.byIBAN[normID(a.IBAN)] = a.ID
			m.ids[a.ID] = append(m.ids[a.ID], a.IBAN)
		}
		for _, p := range a.Pans {
			m.byPAN[idKey(p)] = a.ID
			m.ids[a.ID] = append(m.ids[a.ID], p)
			if lk := looseKey(p); lk != "" {
				m.byLoose[lk] = append(m.byLoose[lk], a.ID)
			}
		}
	}
	return m
}

func (m *ownMatcher) lookup(id string) string {
	if id == "" {
		return ""
	}
	if isIBANLike(id) {
		return m.byIBAN[normID(id)]
	}
	if k := reliableKey(id); k != "" {
		if acc, ok := m.byPAN[k]; ok {
			return acc
		}
	}
	if lk := looseKey(id); lk != "" && len(m.byLoose[lk]) == 1 {
		return m.byLoose[lk][0]
	}
	return ""
}

// fromAliasAccts — свой счёт по счетам и картам алиаса: берётся, только если среди них ровно один наш счёт, кроме счёта операции.
func (m *ownMatcher) fromAliasAccts(selfAccount string, list []AliasAcct) string {
	found := ""
	for _, a := range list {
		acc := m.lookup(a.ID)
		if acc == "" || acc == selfAccount {
			continue
		}
		if found != "" && found != acc {
			return "" // несколько наших счетов у одного алиаса — какой именно, неизвестно
		}
		found = acc
	}
	return found
}

// own — id нашего счёта, на который/с которого ушла операция ("" — внешний контрагент или номер не указан).
func (m *ownMatcher) own(selfAccount, counterIBAN, counterName, description string) string {
	cands := []string{counterIBAN}
	for _, text := range []string{counterName, description} {
		for _, g := range reCardInText.FindAllStringSubmatch(text, -1) {
			cands = append(cands, g[1]+"****"+g[2])
		}
	}
	for _, c := range cands {
		if acc := m.lookup(c); acc != "" && acc != selfAccount {
			return acc
		}
	}
	return ""
}

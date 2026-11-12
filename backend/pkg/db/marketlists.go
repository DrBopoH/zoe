package db

import (
	"database/sql"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Свои списки бумаг для рынка (например «icu» — что сейчас доступно к покупке в брокере). Это данные пользователя,
// а не скачанный кеш, поэтому лежат в core.db и переживают «стереть кеш».
const marketListsSchema = `
CREATE TABLE IF NOT EXISTS market_lists (
	name TEXT PRIMARY KEY,
	isins TEXT NOT NULL DEFAULT '', -- ISIN через пробел
	updated_at INTEGER NOT NULL DEFAULT 0
);`

var marketListName = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

func (s *Storage) initMarketLists() error {
	_, err := s.coreDB.Exec(marketListsSchema)
	return err
}

// Записи списка: «ISIN» или «ISIN=цена» (цена с рынка брокера, чистая сумма за бумагу; по ней считается доходность).

// MarketList — ISIN списка по имени (нет списка — пусто) и известные цены.
func (s *Storage) MarketList(name string) ([]string, map[string]float64, error) {
	if !marketListName.MatchString(name) {
		return nil, nil, errors.New("имя списка: a-z, 0-9, _ и -")
	}
	var raw string
	err := s.coreDB.QueryRow(`SELECT isins FROM market_lists WHERE name = ?`, name).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, err
	}
	ids, prices := []string{}, map[string]float64{}
	for _, tok := range strings.Fields(raw) {
		id, price, _ := strings.Cut(tok, "=")
		ids = append(ids, id)
		if v, err := strconv.ParseFloat(price, 64); err == nil && v > 0 {
			prices[id] = v
		}
	}
	return ids, prices, nil
}

// SaveMarketList заменяет список целиком: ISIN приводятся к верхнему регистру, повторы убираются, цена — только положительная.
func (s *Storage) SaveMarketList(name string, isins []string, prices map[string]float64, nowUnix int64) ([]string, map[string]float64, error) {
	if !marketListName.MatchString(name) {
		return nil, nil, errors.New("имя списка: a-z, 0-9, _ и -")
	}
	seen := map[string]bool{}
	clean := []string{}
	for _, id := range isins {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id != "" && !seen[id] {
			seen[id] = true
			clean = append(clean, id)
		}
	}
	sort.Strings(clean)
	kept := map[string]float64{}
	toks := make([]string, 0, len(clean))
	for _, id := range clean {
		tok := id
		for k, v := range prices {
			if strings.ToUpper(strings.TrimSpace(k)) == id && v > 0 {
				kept[id] = v
				tok = id + "=" + strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
		toks = append(toks, tok)
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`
		INSERT INTO market_lists(name, isins, updated_at) VALUES(?,?,?)
		ON CONFLICT(name) DO UPDATE SET isins = excluded.isins, updated_at = excluded.updated_at`,
		name, strings.Join(toks, " "), nowUnix)
	return clean, kept, err
}

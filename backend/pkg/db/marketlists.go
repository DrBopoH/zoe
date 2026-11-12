package db

import (
	"database/sql"
	"errors"
	"regexp"
	"sort"
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

// MarketList — ISIN списка по имени (нет списка — пусто).
func (s *Storage) MarketList(name string) ([]string, error) {
	if !marketListName.MatchString(name) {
		return nil, errors.New("имя списка: a-z, 0-9, _ и -")
	}
	var raw string
	err := s.coreDB.QueryRow(`SELECT isins FROM market_lists WHERE name = ?`, name).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return strings.Fields(raw), nil
}

// SaveMarketList заменяет список целиком: ISIN приводятся к верхнему регистру, повторы убираются.
func (s *Storage) SaveMarketList(name string, isins []string, nowUnix int64) ([]string, error) {
	if !marketListName.MatchString(name) {
		return nil, errors.New("имя списка: a-z, 0-9, _ и -")
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
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`
		INSERT INTO market_lists(name, isins, updated_at) VALUES(?,?,?)
		ON CONFLICT(name) DO UPDATE SET isins = excluded.isins, updated_at = excluded.updated_at`,
		name, strings.Join(clean, " "), nowUnix)
	return clean, err
}

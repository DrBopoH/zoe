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
	if _, err := s.coreDB.Exec(marketListsSchema); err != nil {
		return err
	}
	return s.initMarketCurves()
}

// Записи списка: «ISIN», «ISIN=цена», «ISIN~доходность» или «ISIN=цена~доходность». Цена — с экрана брокера (за бумагу, с накопленным
// купоном), доходность — эффективная годовая в % (показанная брокером). Нужно хоть что-то одно, чтобы оценить бумагу по рынку.

type MarketEntry struct {
	Price float64
	YTM   float64
}

func parseMarketToken(tok string) (string, MarketEntry) {
	id, rest, _ := strings.Cut(tok, "=")
	var e MarketEntry
	if rest == "" && strings.Contains(id, "~") {
		id, rest, _ = strings.Cut(id, "~")
		rest = "~" + rest
	}
	price, ytm, hasYTM := strings.Cut(rest, "~")
	if v, err := strconv.ParseFloat(price, 64); err == nil && v > 0 {
		e.Price = v
	}
	if hasYTM {
		if v, err := strconv.ParseFloat(ytm, 64); err == nil && v > 0 && v < 100 {
			e.YTM = v
		}
	}
	return id, e
}

// MarketList — ISIN списка по имени (нет списка — пусто) и известные цены и доходности.
func (s *Storage) MarketList(name string) ([]string, map[string]MarketEntry, error) {
	if !marketListName.MatchString(name) {
		return nil, nil, errors.New("имя списка: a-z, 0-9, _ и -")
	}
	var raw string
	err := s.coreDB.QueryRow(`SELECT isins FROM market_lists WHERE name = ?`, name).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, err
	}
	ids, entries := []string{}, map[string]MarketEntry{}
	for _, tok := range strings.Fields(raw) {
		id, e := parseMarketToken(tok)
		ids = append(ids, id)
		if e.Price > 0 || e.YTM > 0 {
			entries[id] = e
		}
	}
	return ids, entries, nil
}

// SaveMarketList заменяет список целиком: ISIN приводятся к верхнему регистру, повторы убираются, цена и доходность — только
// положительные и только у бумаг из списка.
func (s *Storage) SaveMarketList(name string, isins []string, entries map[string]MarketEntry, nowUnix int64) ([]string, map[string]MarketEntry, error) {
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
	byID := map[string]MarketEntry{}
	for k, v := range entries {
		byID[strings.ToUpper(strings.TrimSpace(k))] = v
	}
	kept := map[string]MarketEntry{}
	toks := make([]string, 0, len(clean))
	for _, id := range clean {
		e := byID[id]
		if !(e.Price > 0) {
			e.Price = 0
		}
		if !(e.YTM > 0 && e.YTM < 100) {
			e.YTM = 0
		}
		tok := id
		if e.Price > 0 {
			tok += "=" + strconv.FormatFloat(e.Price, 'f', -1, 64)
		}
		if e.YTM > 0 {
			tok += "~" + strconv.FormatFloat(e.YTM, 'f', -1, 64)
		}
		if e.Price > 0 || e.YTM > 0 {
			kept[id] = e
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

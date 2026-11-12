package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	_ "modernc.org/sqlite"
)

// Storage держит ДВЕ независимые базы:
//
//	cacheDB — банковский кеш (accounts, transactions). Можно удалить в любой момент.
//	coreDB  — статика: лица, привязка алиасов, запомненная структура, теневые баны,
//	          ручные правки счетов. Парсеры и синхронизация сюда НЕ пишут напрямую.
//
// Единственный путь «кеш → статика» — syncAliases(): только для алиасов,
// уже привязанных к лицу, только то, что кеш подтверждает, и мимо теневых банов.
type Storage struct {
	cacheDB *sql.DB
	coreDB  *sql.DB
	cacheMu sync.Mutex // сериализует записи в кеш
	coreMu  sync.Mutex // сериализует записи в статику

	ver atomic.Int64 // растёт при любом изменении данных (кеш или статика)
}

// Version — счётчик изменений. Фронту достаточно опрашивать его вместо
// перезагрузки всех транзакций каждые 3 секунды.
func (s *Storage) Version() int64 { return s.ver.Load() }
func (s *Storage) bump()          { s.ver.Add(1) }

// Close закрывает обе базы (корректно сбрасывает WAL).
func (s *Storage) Close() error {
	e1 := s.cacheDB.Close()
	e2 := s.coreDB.Close()
	if e1 != nil {
		return e1
	}
	return e2
}

func openDB(path string) (*sql.DB, error) {
	// PRAGMA через DSN применяются к КАЖДОМУ соединению пула
	// (db.Exec("PRAGMA ...") действовал бы только на одно).
	dsn := path + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func NewStorage(cacheDBPath, coreDBPath string) (*Storage, error) {
	cacheDB, err := openDB(cacheDBPath)
	if err != nil {
		return nil, fmt.Errorf("cache db: %w", err)
	}
	coreDB, err := openDB(coreDBPath)
	if err != nil {
		return nil, fmt.Errorf("core db: %w", err)
	}

	s := &Storage{cacheDB: cacheDB, coreDB: coreDB}
	if err := s.initCache(); err != nil {
		return nil, err
	}
	if err := s.initCore(); err != nil {
		return nil, err
	}
	s.backfillIbanPans()
	s.syncAliases(nil)
	return s, nil
}

// ───────────────────────── helpers ─────────────────────────

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

func tableExists(q querier, name string) bool {
	rows, err := q.Query(`SELECT 1 FROM sqlite_master WHERE type='table' AND name=?`, name)
	if err != nil {
		return false
	}
	defer rows.Close()
	return rows.Next()
}

func hasColumn(q querier, table, col string) bool {
	rows, err := q.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk) == nil && strings.EqualFold(name, col) {
			return true
		}
	}
	return false
}

// ───────────────────────── cache.db ─────────────────────────

func (s *Storage) initCache() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			name TEXT,
			balance REAL,
			credit_limit REAL DEFAULT 0,
			type TEXT,
			currency_code INTEGER,
			iban TEXT,
			masked_pan TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			account_id TEXT,
			time INTEGER,
			description TEXT,
			comment TEXT,
			counter_edrpou TEXT,
			counter_iban TEXT,
			counter_name TEXT,
			mcc INTEGER,
			original_mcc INTEGER,
			amount REAL,
			operation_amount REAL,
			currency_code INTEGER,
			commission_rate REAL,
			cashback_amount REAL,
			is_hold BOOLEAN
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tx_time ON transactions(time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tx_account ON transactions(account_id)`,
		coverageSchema,
	}
	for _, q := range stmts {
		if _, err := s.cacheDB.Exec(q); err != nil {
			return fmt.Errorf("cache init: %w", err)
		}
	}

	// Миграции старых кеш-файлов (кеш одноразовый, но так бережнее).
	if !hasColumn(s.cacheDB, "accounts", "extra_pans") {
		if _, err := s.cacheDB.Exec(`ALTER TABLE accounts ADD COLUMN extra_pans TEXT DEFAULT ''`); err != nil {
			return err
		}
	}
	for _, col := range []string{"owner TEXT DEFAULT ''", "opened_at INTEGER DEFAULT 0", "closed_at INTEGER DEFAULT 0"} {
		if !hasColumn(s.cacheDB, "accounts", strings.Fields(col)[0]) {
			if _, err := s.cacheDB.Exec(`ALTER TABLE accounts ADD COLUMN ` + col); err != nil {
				return err
			}
		}
	}
	if !hasColumn(s.cacheDB, "accounts", "source") {
		if _, err := s.cacheDB.Exec(`ALTER TABLE accounts ADD COLUMN source TEXT DEFAULT 'api'`); err != nil {
			return err
		}
		_, _ = s.cacheDB.Exec(`UPDATE accounts SET source='import'
			WHERE id LIKE 'privat\_%' ESCAPE '\' OR id LIKE 'raif\_%' ESCAPE '\' OR id LIKE 'csv\_%' ESCAPE '\'`)
	}

	// Раньше masked_pan хранил «PAN1, PAN2» одной строкой → главный + хвост.
	rows, err := s.cacheDB.Query(`SELECT id, masked_pan FROM accounts WHERE masked_pan LIKE '%,%'`)
	if err != nil {
		return err
	}
	type fix struct{ id, main, extra string }
	var fixes []fix
	for rows.Next() {
		var id, mp string
		if rows.Scan(&id, &mp) == nil {
			p := mergePans(splitPans(mp))
			if len(p) > 0 {
				fixes = append(fixes, fix{id, p[0], strings.Join(p[1:], ", ")})
			}
		}
	}
	rows.Close()
	for _, f := range fixes {
		_, _ = s.cacheDB.Exec(`UPDATE accounts SET masked_pan=?, extra_pans=? WHERE id=?`, f.main, f.extra, f.id)
	}
	return nil
}

// ───────────────────────── core.db ─────────────────────────

const coreSchema = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'DONOR',
	avatar_url TEXT
);
-- Лицо знает ТОЛЬКО названия своих алиасов. Всё остальное — через алиас.
CREATE TABLE IF NOT EXISTS user_aliases (
	alias_key TEXT PRIMARY KEY,          -- lower(trim(alias)), Unicode-безопасно (считает Go)
	alias     TEXT NOT NULL,             -- как показывать
	user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE
);
-- Запомненная структура алиаса: alias → IBAN → PAN. Живёт и у «бесхозных» алиасов.
CREATE TABLE IF NOT EXISTS alias_accounts (
	alias_key TEXT NOT NULL,
	alias     TEXT NOT NULL,
	account_identifier TEXT NOT NULL,
	parent_identifier  TEXT NOT NULL DEFAULT '',   -- для PAN: его IBAN
	PRIMARY KEY (alias_key, account_identifier)
);
-- Теневой бан: «кеш, не подхватывай это для алиаса, пока он привязан к лицу».
-- Удаляется при отвязке алиаса от лица или ручном возврате связи в модалке.
CREATE TABLE IF NOT EXISTS alias_bans (
	alias_key TEXT NOT NULL,
	account_identifier TEXT NOT NULL,
	PRIMARY KEY (alias_key, account_identifier)
);
-- Глобальный факт «эта карта принадлежит этому IBAN» — един для всех алиасов и лиц.
CREATE TABLE IF NOT EXISTS iban_pans (
	pan_key TEXT PRIMARY KEY,
	pan     TEXT NOT NULL,
	iban    TEXT NOT NULL
);
-- Ручные правки счёта (лимит, IBAN, PAN) — это пользовательские данные, им не место в кеше.
CREATE TABLE IF NOT EXISTS account_overrides (
	account_id TEXT PRIMARY KEY,
	iban TEXT NOT NULL DEFAULT '',
	pan  TEXT NOT NULL DEFAULT '',
	credit_limit REAL
);
`

func (s *Storage) initCore() error {
	s.coreMu.Lock()
	defer s.coreMu.Unlock()

	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY, name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'DONOR', avatar_url TEXT)`); err != nil {
		return err
	}

	// ── миграция со старой схемы ──
	legacyAliases := tableExists(tx, "user_aliases") && !hasColumn(tx, "user_aliases", "alias_key")
	legacyLinks := tableExists(tx, "alias_accounts") && !hasColumn(tx, "alias_accounts", "alias_key")
	legacyUA := tableExists(tx, "user_accounts")

	type pair struct{ a, b, c string }
	var oldAliases, oldLinks, oldUA []pair

	read := func(q string, n int) ([]pair, error) {
		rows, err := tx.Query(q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []pair
		for rows.Next() {
			var p pair
			var err error
			if n == 2 {
				err = rows.Scan(&p.a, &p.b)
			} else {
				err = rows.Scan(&p.a, &p.b, &p.c)
			}
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, rows.Err()
	}

	if legacyAliases {
		// только алиасы существующих лиц (иначе FK уронит миграцию)
		if oldAliases, err = read(`SELECT ua.user_id, ua.alias FROM user_aliases ua JOIN users u ON u.id = ua.user_id`, 2); err != nil {
			return err
		}
	}
	if legacyLinks {
		if oldLinks, err = read(`SELECT alias, account_identifier FROM alias_accounts`, 2); err != nil {
			return err
		}
	}
	if legacyUA && hasColumn(tx, "user_accounts", "note") {
		parentCol := `''`
		if hasColumn(tx, "user_accounts", "parent_identifier") {
			parentCol = `COALESCE(parent_identifier,'')`
		}
		if oldUA, err = read(`SELECT COALESCE(note,''), account_identifier, `+parentCol+` FROM user_accounts`, 3); err != nil {
			return err
		}
	}

	if legacyAliases {
		if _, err := tx.Exec(`ALTER TABLE user_aliases RENAME TO user_aliases_legacy`); err != nil {
			return err
		}
	}
	if legacyLinks {
		if _, err := tx.Exec(`ALTER TABLE alias_accounts RENAME TO alias_accounts_legacy`); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(coreSchema); err != nil {
		return fmt.Errorf("core schema: %w", err)
	}

	for _, p := range oldAliases {
		if k := aliasKey(p.b); k != "" {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO user_aliases(alias_key, alias, user_id) VALUES(?,?,?)`, k, strings.TrimSpace(p.b), p.a); err != nil {
				return err
			}
		}
	}
	parentOf := map[string]string{}
	for _, p := range oldUA {
		if p.c != "" {
			parentOf[p.b] = p.c
		}
	}
	insLink := func(alias, id, parent string) error {
		k := aliasKey(alias)
		if k == "" || strings.TrimSpace(id) == "" {
			return nil
		}
		if isIBANLike(id) {
			parent = ""
		}
		_, err := tx.Exec(`INSERT OR IGNORE INTO alias_accounts(alias_key, alias, account_identifier, parent_identifier) VALUES(?,?,?,?)`,
			k, strings.TrimSpace(alias), strings.TrimSpace(id), parent)
		return err
	}
	for _, p := range oldLinks {
		if err := insLink(p.a, p.b, parentOf[p.b]); err != nil {
			return err
		}
	}
	for _, p := range oldUA {
		if err := insLink(p.a, p.b, p.c); err != nil {
			return err
		}
	}
	if legacyUA {
		if _, err := tx.Exec(`ALTER TABLE user_accounts RENAME TO user_accounts_legacy`); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if err := s.initManual(); err != nil {
		return fmt.Errorf("manual schema: %w", err)
	}
	if err := s.initBinance(); err != nil {
		return fmt.Errorf("binance schema: %w", err)
	}
	// ручные даты жизни счёта (NULL — «считай по выписке»)
	for _, col := range []string{"opened_at INTEGER", "closed_at INTEGER"} {
		if !hasColumn(s.coreDB, "account_overrides", strings.Fields(col)[0]) {
			if _, err := s.coreDB.Exec(`ALTER TABLE account_overrides ADD COLUMN ` + col); err != nil {
				return err
			}
		}
	}
	if legacyAliases || legacyLinks || legacyUA {
		log.Printf("[DB] core.db мигрирована на схему «алиас → структура» (старые таблицы сохранены как *_legacy)")
	}
	return nil
}

// WipeCache сносит ТОЛЬКО кеш. Статика остаётся нетронутой.
func (s *Storage) WipeCache() error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	if _, err := s.cacheDB.Exec(`DELETE FROM transactions`); err != nil {
		return err
	}
	if _, err := s.cacheDB.Exec(`DELETE FROM sync_coverage`); err != nil {
		return err
	}
	_, err := s.cacheDB.Exec(`DELETE FROM accounts`)
	return err
}

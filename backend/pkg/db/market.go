package db

import (
	"economic/pkg/market"
	"strings"
)

// Рыночные данные по облигациям (ОВДП и US Treasury) из публичных источников — кеш: стереть и скачать заново можно
// в любой момент. Своё ручное (core.bonds) всегда главнее скачанного: см. bondTerms.
const marketCacheSchema = `
CREATE TABLE IF NOT EXISTS market_bonds (
	source TEXT NOT NULL,             -- nbu | ust
	id TEXT NOT NULL,                 -- ISIN или CUSIP
	name TEXT NOT NULL DEFAULT '',
	currency TEXT NOT NULL DEFAULT '',
	maturity TEXT NOT NULL DEFAULT '', -- YYYY-MM-DD
	issued TEXT NOT NULL DEFAULT '',
	coupon_rate REAL NOT NULL DEFAULT 0,
	nominal REAL NOT NULL DEFAULT 0,
	yield REAL NOT NULL DEFAULT 0,
	last_auction TEXT NOT NULL DEFAULT '',
	updated_at INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (source, id)
);
CREATE INDEX IF NOT EXISTS idx_market_maturity ON market_bonds(maturity);
CREATE TABLE IF NOT EXISTS market_companies (
	ticker TEXT NOT NULL PRIMARY KEY, -- в записи SEC и Yahoo (BRK-B)
	name TEXT NOT NULL DEFAULT '',
	exchange TEXT NOT NULL DEFAULT '',
	cik INTEGER NOT NULL DEFAULT 0,
	sector TEXT NOT NULL DEFAULT '',
	industry TEXT NOT NULL DEFAULT '',
	country TEXT NOT NULL DEFAULT '',
	ipo_year INTEGER NOT NULL DEFAULT 0,
	market_cap REAL NOT NULL DEFAULT 0,
	price REAL NOT NULL DEFAULT 0,
	updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_market_companies_cap ON market_companies(market_cap DESC);
`

// SaveMarketBonds кладёт бумаги в кеш. Пришедшее пустое (нет ставки, даты, доходности) не затирает уже известное:
// реестр и результаты аукционов дают разные поля одной бумаги. Старые, уже погашенные бумаги остаются.
func (s *Storage) SaveMarketBonds(list []market.Security, nowUnix int64) error {
	if len(list) == 0 {
		return nil
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, b := range list {
		if b.Source == "" || b.ID == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO market_bonds(source, id, name, currency, maturity, issued, coupon_rate, nominal, yield, last_auction, updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(source, id) DO UPDATE SET
				name         = CASE WHEN excluded.name <> '' THEN excluded.name ELSE market_bonds.name END,
				currency     = CASE WHEN excluded.currency <> '' THEN excluded.currency ELSE market_bonds.currency END,
				maturity     = CASE WHEN excluded.maturity <> '' THEN excluded.maturity ELSE market_bonds.maturity END,
				issued       = CASE WHEN excluded.issued <> '' THEN excluded.issued ELSE market_bonds.issued END,
				coupon_rate  = CASE WHEN excluded.coupon_rate > 0 THEN excluded.coupon_rate ELSE market_bonds.coupon_rate END,
				nominal      = CASE WHEN excluded.nominal > 0 THEN excluded.nominal ELSE market_bonds.nominal END,
				yield        = CASE WHEN excluded.yield > 0 THEN excluded.yield ELSE market_bonds.yield END,
				last_auction = CASE WHEN excluded.last_auction <> '' THEN excluded.last_auction ELSE market_bonds.last_auction END,
				updated_at   = excluded.updated_at`,
			b.Source, strings.ToUpper(b.ID), b.Name, b.Currency, b.Maturity, b.Issued, b.CouponRate, b.Nominal, b.Yield, b.LastAuction, nowUnix); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// MarketBonds — скачанные бумаги источника (source пустой — все), ближайшее погашение первым.
func (s *Storage) MarketBonds(source string) ([]market.Security, error) {
	q := `SELECT source, id, name, currency, maturity, issued, coupon_rate, nominal, yield, last_auction FROM market_bonds`
	var args []any
	if source != "" {
		q += ` WHERE source = ?`
		args = append(args, source)
	}
	q += ` ORDER BY maturity, id`
	rows, err := s.cacheDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []market.Security{}
	for rows.Next() {
		var b market.Security
		if err := rows.Scan(&b.Source, &b.ID, &b.Name, &b.Currency, &b.Maturity, &b.Issued, &b.CouponRate, &b.Nominal, &b.Yield, &b.LastAuction); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

// SaveMarketCompanies заменяет список компаний целиком: каждый опрос даёт полный снимок (кто сошёл с биржи, того в списке больше нет).
// Пустой список ничего не меняет: сбой источника не должен стирать прошлый снимок.
func (s *Storage) SaveMarketCompanies(list []market.Company, nowUnix int64) error {
	if len(list) == 0 {
		return nil
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM market_companies`); err != nil {
		return err
	}
	for _, c := range list {
		if c.Ticker == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT OR REPLACE INTO market_companies(ticker, name, exchange, cik, sector, industry, country, ipo_year, market_cap, price, updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			strings.ToUpper(c.Ticker), c.Name, c.Exchange, c.CIK, c.Sector, c.Industry, c.Country, c.IPOYear, c.MarketCap, c.Price, nowUnix); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// MarketCompanies — список компаний, крупные по капитализации первыми. q — часть тикера или названия (без учёта регистра), limit 0 — все.
func (s *Storage) MarketCompanies(q string, limit int) ([]market.Company, error) {
	query := `SELECT ticker, name, exchange, cik, sector, industry, country, ipo_year, market_cap, price FROM market_companies`
	var args []any
	if q = strings.ToUpper(strings.TrimSpace(q)); q != "" {
		query += ` WHERE instr(UPPER(ticker), ?) > 0 OR instr(UPPER(name), ?) > 0`
		args = append(args, q, q)
	}
	query += ` ORDER BY market_cap DESC, ticker`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.cacheDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []market.Company{}
	for rows.Next() {
		var c market.Company
		if err := rows.Scan(&c.Ticker, &c.Name, &c.Exchange, &c.CIK, &c.Sector, &c.Industry, &c.Country, &c.IPOYear, &c.MarketCap, &c.Price); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// bondTerms — условия облигаций для расчёта: справочник пользователя (core.bonds) поверх скачанных ОВДП.
// Скачанная бумага годится для расчёта купонов, если у неё есть дата погашения и ставка > 0 (иначе оценка идёт
// по себестоимости, как и раньше). Валюта не гривна — не берётся: расчёт счёта ICU гривневый.
func (s *Storage) bondTerms() ([]Bond, error) {
	own, err := s.ListBonds()
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(own))
	for _, b := range own {
		have[b.ISIN] = true
	}
	mk, err := s.MarketBonds(market.SourceNBU)
	if err != nil {
		return nil, err
	}
	for _, m := range mk {
		if have[m.ID] || m.Maturity == "" || m.CouponRate <= 0 || (m.Currency != "" && m.Currency != "UAH") {
			continue
		}
		nom := m.Nominal
		if nom <= 0 {
			nom = 1000
		}
		own = append(own, Bond{ISIN: m.ID, Name: m.Name, Maturity: m.Maturity, CouponRate: m.CouponRate, Nominal: nom})
	}
	return own, nil
}

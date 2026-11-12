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

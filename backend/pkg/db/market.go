package db

import (
	"database/sql"
	"economic/pkg/market"
	"strings"
	"time"
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
CREATE TABLE IF NOT EXISTS stock_prices (
	ticker TEXT NOT NULL,
	d TEXT NOT NULL,                  -- первое число месяца, YYYY-MM-01
	close REAL NOT NULL,              -- закрытие месяца с поправкой на сплиты
	adj REAL NOT NULL,                -- и на дивиденды
	PRIMARY KEY (ticker, d)
);
CREATE TABLE IF NOT EXISTS stock_splits (
	ticker TEXT NOT NULL,
	d TEXT NOT NULL,                  -- день сплита
	ratio REAL NOT NULL,              -- во сколько раз стало больше акций (4:1 — 4, обратный 1:10 — 0.1)
	PRIMARY KEY (ticker, d)
);
CREATE TABLE IF NOT EXISTS stock_split_known (
	ticker TEXT NOT NULL PRIMARY KEY  -- сплиты тикера скачаны (у тикера без сплитов строк в stock_splits нет, но отметка есть)
);
CREATE TABLE IF NOT EXISTS stock_facts (
	tag TEXT NOT NULL,                -- us-gaap/CommonStockSharesOutstanding
	period TEXT NOT NULL,             -- кадр SEC: CY2024Q4I, CY2023
	cik INTEGER NOT NULL,
	end TEXT NOT NULL,                -- дата, на которую значение относится у этой компании
	val REAL NOT NULL,
	PRIMARY KEY (tag, period, cik)
);
CREATE INDEX IF NOT EXISTS idx_stock_facts_cik ON stock_facts(cik, end);
CREATE TABLE IF NOT EXISTS stock_frames (
	tag TEXT NOT NULL,
	period TEXT NOT NULL,
	at INTEGER NOT NULL,              -- когда кадр скачан (unix)
	n INTEGER NOT NULL DEFAULT 0,     -- сколько компаний в нём
	PRIMARY KEY (tag, period)
);
CREATE TABLE IF NOT EXISTS stock_fetched (
	ticker TEXT NOT NULL PRIMARY KEY,
	at INTEGER NOT NULL               -- когда история скачана (unix)
);
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

// SaveStockPrices заменяет историю и сплиты одного тикера целиком (Yahoo пересчитывает прошлые цены при сплитах и дивидендах) и помечает время загрузки.
// Пустая история ничего не меняет.
func (s *Storage) SaveStockPrices(ticker string, bars []market.PriceBar, splits []market.Split, nowUnix int64) error {
	if len(bars) == 0 {
		return nil
	}
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{`DELETE FROM stock_prices WHERE ticker = ?`, `DELETE FROM stock_splits WHERE ticker = ?`} {
		if _, err := tx.Exec(q, ticker); err != nil {
			return err
		}
	}
	for _, b := range bars {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO stock_prices(ticker, d, close, adj) VALUES(?,?,?,?)`, ticker, b.Date, b.Close, b.Adj); err != nil {
			return err
		}
	}
	for _, sp := range splits {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO stock_splits(ticker, d, ratio) VALUES(?,?,?)`, ticker, sp.Date, sp.Ratio); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO stock_split_known(ticker) VALUES(?)`, ticker); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO stock_fetched(ticker, at) VALUES(?,?)`, ticker, nowUnix); err != nil {
		return err
	}
	return tx.Commit()
}

// StockSplits — сплиты тикера по возрастанию даты.
func (s *Storage) StockSplits(ticker string) ([]market.Split, error) {
	rows, err := s.cacheDB.Query(`SELECT d, ratio FROM stock_splits WHERE ticker = ? ORDER BY d`, strings.ToUpper(strings.TrimSpace(ticker)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []market.Split{}
	for rows.Next() {
		var sp market.Split
		if err := rows.Scan(&sp.Date, &sp.Ratio); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// SaveStockFrame заменяет кадр отчётности SEC целиком (по CIK) и помечает время загрузки; пустой кадр тоже помечается — период просто без значений.
func (s *Storage) SaveStockFrame(req market.FrameRequest, facts []market.FrameFact, nowUnix int64) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM stock_facts WHERE tag = ? AND period = ?`, req.Tag, req.Period); err != nil {
		return err
	}
	for _, f := range facts {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO stock_facts(tag, period, cik, end, val) VALUES(?,?,?,?,?)`, req.Tag, req.Period, f.CIK, f.End, f.Val); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO stock_frames(tag, period, at, n) VALUES(?,?,?,?)`, req.Tag, req.Period, nowUnix, len(facts)); err != nil {
		return err
	}
	return tx.Commit()
}

// StockFramesHave — какие кадры отчётности скачаны: ключ «тег|период» → когда (unix).
func (s *Storage) StockFramesHave() (map[string]int64, error) {
	rows, err := s.cacheDB.Query(`SELECT tag, period, at FROM stock_frames`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	have := map[string]int64{}
	for rows.Next() {
		var tag, period string
		var at int64
		if err := rows.Scan(&tag, &period, &at); err != nil {
			return nil, err
		}
		have[tag+"|"+period] = at
	}
	return have, rows.Err()
}

// StockFetchedAt — у каких тикеров скачана история с отметкой о сплитах: тикер → когда (unix).
func (s *Storage) StockFetchedAt() (map[string]int64, error) {
	rows, err := s.cacheDB.Query(`SELECT f.ticker, f.at FROM stock_fetched f JOIN stock_split_known k ON k.ticker = f.ticker`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var t string
		var at int64
		if err := rows.Scan(&t, &at); err != nil {
			return nil, err
		}
		out[t] = at
	}
	return out, rows.Err()
}

// StockFramesTodo — какие из нужных кадров ещё не скачаны, а свежие (Recent: SEC дописывает отчёты за последние кварталы) — ещё и устарели старше maxAge.
func (s *Storage) StockFramesTodo(all []market.FrameRequest, maxAge time.Duration, nowUnix int64) ([]market.FrameRequest, error) {
	have, err := s.StockFramesHave()
	if err != nil {
		return nil, err
	}
	var todo []market.FrameRequest
	for _, r := range all {
		at, ok := have[r.Key()]
		if !ok || (r.Recent && at < nowUnix-int64(maxAge/time.Second)) {
			todo = append(todo, r)
		}
	}
	return todo, nil
}

// StockShares — число акций компании по CIK из сохранённых кадров (баланс и обложка отчёта) по возрастанию даты.
func (s *Storage) StockShares(cik int) ([]market.ShareFact, error) {
	rows, err := s.cacheDB.Query(`SELECT tag, period, end, val FROM stock_facts WHERE cik = ? AND tag IN (?, ?) ORDER BY end, period`, cik, market.TagSharesBalance, market.TagSharesCover)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []market.ShareFact{}
	for rows.Next() {
		var f market.ShareFact
		if err := rows.Scan(&f.Tag, &f.Period, &f.End, &f.Val); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// StockPrices — месячная история тикера по возрастанию даты и время её загрузки (0 — истории нет).
func (s *Storage) StockPrices(ticker string) ([]market.PriceBar, int64, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	rows, err := s.cacheDB.Query(`SELECT d, close, adj FROM stock_prices WHERE ticker = ? ORDER BY d`, ticker)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	bars := []market.PriceBar{}
	for rows.Next() {
		b := market.PriceBar{Ticker: ticker}
		if err := rows.Scan(&b.Date, &b.Close, &b.Adj); err != nil {
			return nil, 0, err
		}
		bars = append(bars, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var at int64
	err = s.cacheDB.QueryRow(`SELECT at FROM stock_fetched WHERE ticker = ?`, ticker).Scan(&at)
	if err != nil && err != sql.ErrNoRows {
		return nil, 0, err
	}
	return bars, at, nil
}

// StockTickersToFetch — тикеры n крупнейших по капитализации компаний, чья история не скачана, старше maxAge или скачана без сплитов; крупные первыми.
func (s *Storage) StockTickersToFetch(n int, maxAge time.Duration, nowUnix int64) ([]string, error) {
	rows, err := s.cacheDB.Query(`
		SELECT c.ticker FROM market_companies c
		LEFT JOIN stock_fetched f ON f.ticker = c.ticker
		LEFT JOIN stock_split_known k ON k.ticker = c.ticker
		WHERE c.ticker IN (SELECT ticker FROM market_companies ORDER BY market_cap DESC, ticker LIMIT ?)
		  AND (f.at IS NULL OR f.at < ? OR k.ticker IS NULL)
		ORDER BY c.market_cap DESC, c.ticker`, n, nowUnix-int64(maxAge/time.Second))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MarketCompany — одна компания списка по тикеру (без учёта регистра).
func (s *Storage) MarketCompany(ticker string) (market.Company, bool, error) {
	var c market.Company
	err := s.cacheDB.QueryRow(`SELECT ticker, name, exchange, cik, sector, industry, country, ipo_year, market_cap, price FROM market_companies WHERE ticker = ?`, strings.ToUpper(strings.TrimSpace(ticker))).
		Scan(&c.Ticker, &c.Name, &c.Exchange, &c.CIK, &c.Sector, &c.Industry, &c.Country, &c.IPOYear, &c.MarketCap, &c.Price)
	if err == sql.ErrNoRows {
		return market.Company{}, false, nil
	}
	return c, err == nil, err
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

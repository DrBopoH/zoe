package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Binance — такой же источник из облака, как банки: всё, что отсюда приходит, — КЕШ (cache.db, одноразовый).
// Счёт «Binance» — обычная строка accounts (type=binance), оценка портфеля по минутам — binance_minutes,
// разбор по монетам для модалки — binance_state. В core.db от Binance нет ничего.

const binanceCacheSchema = `
CREATE TABLE IF NOT EXISTS binance_minutes (
	ts INTEGER PRIMARY KEY,          -- unix-секунды, начало минуты
	v REAL NOT NULL                  -- оценка портфеля, ₴
);
CREATE TABLE IF NOT EXISTS binance_state (k TEXT PRIMARY KEY, v TEXT NOT NULL);
-- найденные движения по счёту (депозиты, выводы, сделки, …): каждый просмотренный период пишется сразу
CREATE TABLE IF NOT EXISTS binance_flows (
	id TEXT PRIMARY KEY,             -- время|монета|сумма|вид|№ повтора
	t INTEGER NOT NULL,              -- мс
	asset TEXT NOT NULL,
	delta REAL NOT NULL,
	kind TEXT NOT NULL
);
-- какие периоды уже просмотрены (stage + key: P2P BUY/SELL, пара сделок), чтобы не качать их заново
CREATE TABLE IF NOT EXISTS binance_coverage (
	stage TEXT NOT NULL,
	key TEXT NOT NULL,
	from_ms INTEGER NOT NULL,
	to_ms INTEGER NOT NULL,
	PRIMARY KEY (stage, key, from_ms, to_ms)
);
`

const (
	BinanceAccountID   = "binance"
	BinanceAccountType = "binance"
)

// legacyBinanceCore — таблицы старой схемы, когда оценки копились в core.db (там им не место).
var legacyBinanceCore = []string{"binance_snapshots", "binance_minutes", "binance_meta"}

func (s *Storage) dropLegacyBinanceCore() error {
	for _, t := range legacyBinanceCore {
		if _, err := s.coreDB.Exec(`DROP TABLE IF EXISTS ` + t); err != nil {
			return err
		}
	}
	return nil
}

// BinanceFlow — движение по счёту Binance (как его отдал API).
type BinanceFlow struct {
	T     int64
	Asset string
	Delta float64
	Kind  string
}

func flowID(f BinanceFlow, occ int) string {
	return fmt.Sprintf("%d|%s|%.12g|%s|%d", f.T, f.Asset, f.Delta, f.Kind, occ)
}

// SaveBinanceWindow кладёт в кеш найденные за период движения и сам период (одной транзакцией): прерванный синк
// продолжится с того места, где остановился. key различает независимые обходы одного этапа (P2P BUY/SELL, пара сделок).
func (s *Storage) SaveBinanceWindow(stage, key string, fromMs, toMs int64, fl []BinanceFlow) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	seen := map[string]int{}
	for _, f := range fl {
		base := flowID(f, 0)
		occ := seen[base]
		seen[base]++
		if _, err := tx.Exec(`INSERT OR IGNORE INTO binance_flows(id,t,asset,delta,kind) VALUES(?,?,?,?,?)`, flowID(f, occ), f.T, f.Asset, f.Delta, f.Kind); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO binance_coverage(stage,key,from_ms,to_ms) VALUES(?,?,?,?)`, stage, key, fromMs, toMs); err != nil {
		return err
	}
	return tx.Commit()
}

// DropBinanceSeries сбрасывает минутный ряд оценки (движения и покрытие периодов остаются): новые движения, например из
// выписки, меняют прошлое, поэтому следующий «Разведать» соберёт ряд заново из кеша движений — дёшево, запросов почти нет.
func (s *Storage) DropBinanceSeries() error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	if _, err := s.cacheDB.Exec(`DELETE FROM binance_minutes`); err != nil {
		return err
	}
	_, err := s.cacheDB.Exec(`DELETE FROM binance_state WHERE k = 'origin'`)
	return err
}

// BinanceCovered — период (или пара сделок: from=to=0) уже просмотрен и лежит в кеше.
func (s *Storage) BinanceCovered(stage, key string, fromMs, toMs int64) bool {
	var n int
	_ = s.cacheDB.QueryRow(`SELECT COUNT(*) FROM binance_coverage WHERE stage=? AND key=? AND from_ms<=? AND to_ms>=?`, stage, key, fromMs, toMs).Scan(&n)
	return n > 0
}

// BinanceFlows — все сохранённые движения.
func (s *Storage) BinanceFlows() ([]BinanceFlow, error) {
	rows, err := s.cacheDB.Query(`SELECT t, asset, delta, kind FROM binance_flows ORDER BY t`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BinanceFlow
	for rows.Next() {
		var f BinanceFlow
		if err := rows.Scan(&f.T, &f.Asset, &f.Delta, &f.Kind); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// BinanceSave — результат опроса Binance для записи в кеш.
type BinanceSave struct {
	Portfolio float64   // оценка портфеля сейчас, ₴
	Start     int64     // unix-секунды первой минуты Vals
	Vals      []float64 // точки минутного ряда
	Replace   bool      // заменить весь ряд (полная история); иначе дописать точки
	Origin    float64   // оценка «до первой точки» (0 — счёт начался с первого движения); хранится при Replace
	Detail    any
}

// SaveBinance одной транзакцией кладёт в кеш счёт с оценкой, точки минутного ряда, операции счёта и JSON-разбор по монетам.
func (s *Storage) SaveBinance(in BinanceSave) error {
	raw, err := json.Marshal(in.Detail)
	if err != nil {
		return err
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if in.Replace {
		if _, err := tx.Exec(`DELETE FROM binance_minutes`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO binance_state(k,v) VALUES('origin',?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, strconv.FormatFloat(in.Origin, 'f', -1, 64)); err != nil {
			return err
		}
	}
	st, err := tx.Prepare(`INSERT OR REPLACE INTO binance_minutes(ts, v) VALUES(?,?)`)
	if err != nil {
		return err
	}
	defer st.Close()
	for i, v := range in.Vals {
		if _, err := st.Exec(in.Start+int64(i)*60, v); err != nil {
			return err
		}
	}
	if err := upsertBinanceAccount(tx, in.Portfolio, raw); err != nil {
		return err
	}
	return tx.Commit()
}

func upsertBinanceAccount(tx *sql.Tx, balance float64, detail []byte) error {
	if _, err := tx.Exec(`
		INSERT INTO accounts (id, name, balance, type, currency_code, iban, masked_pan, extra_pans, source, owner, opened_at)
		VALUES (?, 'Binance', ?, ?, 980, '', '', '', 'api', 'Binance',
			COALESCE((SELECT MIN(ts) FROM binance_minutes), (SELECT MIN(time) FROM transactions WHERE account_id = ?), 0))
		ON CONFLICT(id) DO UPDATE SET balance = excluded.balance, closed_at = 0, opened_at = excluded.opened_at`,
		BinanceAccountID, round2(balance), BinanceAccountType, BinanceAccountID); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO binance_state(k,v) VALUES('detail',?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, string(detail))
	return err
}

// SaveBinanceAccount кладёт в кеш счёт Binance с текущей оценкой и разбором по монетам — самым первым шагом синка,
// чтобы счёт был в «Операционной» ещё до того, как соберётся история.
func (s *Storage) SaveBinanceAccount(portfolio float64, detail any) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsertBinanceAccount(tx, portfolio, raw); err != nil {
		return err
	}
	return tx.Commit()
}

// BinanceFlowRow — сохранённое движение вместе с его id в кеше.
type BinanceFlowRow struct {
	ID string
	BinanceFlow
}

// BinanceTx — внешнее движение как операция счёта Binance; UAH — со знаком, на момент движения.
type BinanceTx struct {
	FlowID string
	T      int64 // мс
	Desc   string
	UAH    float64
}

// BinanceUnvaluedFlows — внешние движения (по видам kinds), которых ещё нет среди операций счёта.
func (s *Storage) BinanceUnvaluedFlows(kinds []string) ([]BinanceFlowRow, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(kinds)), ",")
	args := make([]any, len(kinds))
	for i, k := range kinds {
		args[i] = k
	}
	rows, err := s.cacheDB.Query(`SELECT f.id, f.t, f.asset, f.delta, f.kind FROM binance_flows f
		WHERE f.kind IN (`+ph+`) AND NOT EXISTS (SELECT 1 FROM transactions x WHERE x.id = 'binance_' || f.id) ORDER BY f.t`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BinanceFlowRow
	for rows.Next() {
		var r BinanceFlowRow
		if err := rows.Scan(&r.ID, &r.T, &r.Asset, &r.Delta, &r.Kind); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SaveBinanceTxs кладёт операции счёта Binance сразу после оценки (по одной на движение).
func (s *Storage) SaveBinanceTxs(txs []BinanceTx) error {
	if len(txs) == 0 {
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
	for _, t := range txs {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO transactions (id, account_id, time, description, amount, operation_amount, currency_code, is_hold)
			VALUES (?,?,?,?,?,?,980,0)`, "binance_"+t.FlowID, BinanceAccountID, t.T/1000, t.Desc, round2(t.UAH), round2(t.UAH)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// binanceOrigin — оценка «до начала ряда» (ok == false у рядов старой схемы: тогда открытие первой свечи — первая точка).
func (s *Storage) binanceOrigin() (float64, bool) {
	var v string
	if err := s.cacheDB.QueryRow(`SELECT v FROM binance_state WHERE k='origin'`).Scan(&v); err != nil {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil
}

// BinanceDetail — сохранённый разбор по монетам (пусто, если Binance ещё не опрашивали).
func (s *Storage) BinanceDetail() json.RawMessage {
	var v string
	if err := s.cacheDB.QueryRow(`SELECT v FROM binance_state WHERE k='detail'`).Scan(&v); err != nil {
		return nil
	}
	return json.RawMessage(v)
}

// ───────────────────────── минутный ряд ─────────────────────────

// BinanceSeriesInfo — первая и последняя минута ряда (unix) и число точек.
func (s *Storage) BinanceSeriesInfo() (first, last int64, n int) {
	var f, l sql.NullInt64
	_ = s.cacheDB.QueryRow(`SELECT MIN(ts), MAX(ts), COUNT(*) FROM binance_minutes`).Scan(&f, &l, &n)
	return f.Int64, l.Int64, n
}

// BinanceLastMinute — последняя точка ряда (ok == false, если ряда нет).
func (s *Storage) BinanceLastMinute() (ts int64, v float64, ok bool) {
	err := s.cacheDB.QueryRow(`SELECT ts, v FROM binance_minutes ORDER BY ts DESC LIMIT 1`).Scan(&ts, &v)
	return ts, v, err == nil
}

// BinanceCandles агрегирует ряд в свечи: корзина i — [starts[i], starts[i+1]), последняя — до end (unix).
// Открытие корзины — закрытие предыдущей (ряд непрерывный), поэтому свечи сходятся встык.
// nil — до начала ряда; корзина без точек после начала ряда — плоская свеча на последней цене.
func (s *Storage) BinanceCandles(starts []int64, end int64) ([][4]float64, []bool, error) {
	out := make([][4]float64, len(starts))
	has := make([]bool, len(starts))
	if len(starts) == 0 {
		return out, has, nil
	}
	var prev float64
	havePrev := false
	origin, hasOrigin := s.binanceOrigin()
	var pv sql.NullFloat64
	if err := s.cacheDB.QueryRow(`SELECT v FROM binance_minutes WHERE ts < ? ORDER BY ts DESC LIMIT 1`, starts[0]).Scan(&pv); err == nil && pv.Valid {
		prev, havePrev = pv.Float64, true
	}
	rows, err := s.cacheDB.Query(`SELECT ts, v FROM binance_minutes WHERE ts >= ? AND ts < ? ORDER BY ts`, starts[0], end)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	i := 0
	var cand [4]float64
	started := false
	bucketEnd := func(k int) int64 {
		if k+1 < len(starts) {
			return starts[k+1]
		}
		return end
	}
	for rows.Next() {
		var ts int64
		var v float64
		if err := rows.Scan(&ts, &v); err != nil {
			return nil, nil, err
		}
		for i < len(starts) && ts >= bucketEnd(i) {
			if started {
				out[i], has[i] = cand, true
				prev, havePrev = cand[3], true
				started = false
			} else if havePrev {
				out[i], has[i] = [4]float64{prev, prev, prev, prev}, true
			}
			i++
		}
		if i >= len(starts) {
			break
		}
		if !started {
			o := v
			if havePrev {
				o = prev
			} else if hasOrigin { // первая свеча ряда: до неё в портфеле не было ничего (или была «origin») — скачок виден
				o = origin
			}
			cand = [4]float64{o, maxf(o, v), minf(o, v), v}
			started = true
		} else {
			cand[1], cand[2], cand[3] = maxf(cand[1], v), minf(cand[2], v), v
		}
	}
	for ; i < len(starts); i++ {
		if started {
			out[i], has[i] = cand, true
			prev, havePrev = cand[3], true
			started = false
		} else if havePrev {
			out[i], has[i] = [4]float64{prev, prev, prev, prev}, true
		}
	}
	return out, has, rows.Err()
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Binance: оценка портфеля в гривнах снимается при каждой синхронизации и копится по дням в core.db
// (кеш одноразовый, а история оценок — единственный источник, откуда график берёт движение крипты до сегодняшнего дня).
// Баланс счёта «Binance» — последний снимок; на график он ложится синтетическими внутренними операциями-дельтами дня.

const binanceSchema = `
CREATE TABLE IF NOT EXISTS binance_snapshots (
	day TEXT PRIMARY KEY,            -- ГГГГ-ММ-ДД по местному времени; за день хранится последний снимок
	taken_at INTEGER NOT NULL,
	total REAL NOT NULL,             -- оценка портфеля, ₴
	usdt_uah REAL NOT NULL DEFAULT 0,
	detail TEXT NOT NULL DEFAULT ''  -- JSON: монеты с оценкой и предупреждения
);
-- минутный ряд оценки портфеля (восстановленная история + дописывание при каждом опросе)
CREATE TABLE IF NOT EXISTS binance_minutes (
	ts INTEGER PRIMARY KEY,          -- unix-секунды, начало минуты
	v REAL NOT NULL                  -- оценка, ₴
);
CREATE TABLE IF NOT EXISTS binance_meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);
`

const BinanceAccountID = "binance"

type BinanceSnapshot struct {
	Day     string          `json:"day"`
	TakenAt int64           `json:"taken_at"`
	Total   float64         `json:"total"`
	USDTUAH float64         `json:"usdt_uah"`
	Detail  json.RawMessage `json:"detail,omitempty"`
}

func (s *Storage) initBinance() error {
	_, err := s.coreDB.Exec(binanceSchema)
	return err
}

func localDay(t time.Time) string { return t.In(time.Local).Format("2006-01-02") }

// SaveBinanceSnapshot записывает снимок за день `now` (повторный за тот же день заменяет прежний).
func (s *Storage) SaveBinanceSnapshot(total, usdtUAH float64, detail any, now time.Time) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err = s.coreDB.Exec(`
		INSERT INTO binance_snapshots(day, taken_at, total, usdt_uah, detail) VALUES(?,?,?,?,?)
		ON CONFLICT(day) DO UPDATE SET taken_at=excluded.taken_at, total=excluded.total, usdt_uah=excluded.usdt_uah, detail=excluded.detail`,
		localDay(now), now.Unix(), round2(total), usdtUAH, string(raw))
	return err
}

func (s *Storage) binanceSnapshots() ([]BinanceSnapshot, error) {
	rows, err := s.coreDB.Query(`SELECT day, taken_at, total, usdt_uah, detail FROM binance_snapshots ORDER BY day`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []BinanceSnapshot
	for rows.Next() {
		var b BinanceSnapshot
		var detail string
		if err := rows.Scan(&b.Day, &b.TakenAt, &b.Total, &b.USDTUAH, &detail); err != nil {
			return nil, err
		}
		if detail != "" {
			b.Detail = json.RawMessage(detail)
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

// BinanceLatest — последний снимок (ok == false, если Binance ещё ни разу не снимали).
func (s *Storage) BinanceLatest() (BinanceSnapshot, bool, error) {
	var b BinanceSnapshot
	var detail string
	err := s.coreDB.QueryRow(`SELECT day, taken_at, total, usdt_uah, detail FROM binance_snapshots ORDER BY day DESC LIMIT 1`).
		Scan(&b.Day, &b.TakenAt, &b.Total, &b.USDTUAH, &detail)
	if errors.Is(err, sql.ErrNoRows) {
		return b, false, nil
	}
	if err != nil {
		return b, false, err
	}
	if detail != "" {
		b.Detail = json.RawMessage(detail)
	}
	return b, true, nil
}

// binanceAccount — счёт «Binance» как обычный Account (появляется после первого снимка).
func (s *Storage) binanceAccount() (Account, bool) {
	last, ok, err := s.BinanceLatest()
	if err != nil || !ok {
		return Account{}, false
	}
	var first int64
	_ = s.coreDB.QueryRow(`SELECT MIN(taken_at) FROM binance_snapshots`).Scan(&first)
	if f, _, n := s.BinanceSeriesInfo(); n > 0 && f > 0 && f < first {
		first = f
	}
	return Account{
		ID: BinanceAccountID, Name: "Binance", Balance: last.Total, Type: "crypto", Currency: 980,
		Pans: []string{}, Source: "binance", OpenedAt: first, BankKey: "binance", HasDetail: true,
	}, true
}

// binanceTransactions — синтетические операции: изменение оценки между соседними снимками, на день снимка.
// Первый снимок — «вход» всей суммы (до него Binance в капитале не было). Все они внутренние: не доход и не расход.
func (s *Storage) binanceTransactions() ([]Transaction, error) {
	snaps, err := s.binanceSnapshots()
	if err != nil {
		return nil, err
	}
	if first, _, n := s.BinanceSeriesInfo(); n > 0 && first > 0 {
		return nil, nil // история идёт минутным рядом, дельты снимков больше не нужны
	}
	var out []Transaction
	prev := 0.0
	for i, b := range snaps {
		delta := round2(b.Total - prev)
		prev = b.Total
		if delta == 0 {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", b.Day, time.Local)
		if err != nil {
			continue
		}
		desc := "Binance: изменение оценки"
		if i == 0 {
			desc = "Binance: подключено"
		}
		out = append(out, Transaction{
			ID: "binance_" + b.Day, AccountID: BinanceAccountID,
			Time:        day.Add(12 * time.Hour).Format(time.RFC3339),
			Description: desc, Amount: delta, CurrencyCode: 980, OperationAmount: delta, Internal: true,
		})
	}
	return out, nil
}

// BinanceOverview — последний снимок + сколько дней истории.
type BinanceOverview struct {
	Configured bool             `json:"configured"`
	Snapshot   *BinanceSnapshot `json:"snapshot,omitempty"`
	Days       int              `json:"days"`
}

func (s *Storage) BinanceOverview(configured bool) (BinanceOverview, error) {
	ov := BinanceOverview{Configured: configured}
	last, ok, err := s.BinanceLatest()
	if err != nil {
		return ov, err
	}
	if ok {
		ov.Snapshot = &last
	}
	_ = s.coreDB.QueryRow(`SELECT COUNT(*) FROM binance_snapshots`).Scan(&ov.Days)
	return ov, nil
}

// ───────────────────────── минутный ряд ─────────────────────────

// BinanceSeriesInfo — первая и последняя минута ряда (unix) и число точек.
func (s *Storage) BinanceSeriesInfo() (first, last int64, n int) {
	var f, l sql.NullInt64
	_ = s.coreDB.QueryRow(`SELECT MIN(ts), MAX(ts), COUNT(*) FROM binance_minutes`).Scan(&f, &l, &n)
	return f.Int64, l.Int64, n
}

// BinanceLastMinute — последняя точка ряда (ok == false, если ряда нет).
func (s *Storage) BinanceLastMinute() (ts int64, v float64, ok bool) {
	err := s.coreDB.QueryRow(`SELECT ts, v FROM binance_minutes ORDER BY ts DESC LIMIT 1`).Scan(&ts, &v)
	return ts, v, err == nil
}

func (s *Storage) putMinutes(startUnix int64, vals []float64, replace bool) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if replace {
		if _, err := tx.Exec(`DELETE FROM binance_minutes`); err != nil {
			return err
		}
	}
	st, err := tx.Prepare(`INSERT OR REPLACE INTO binance_minutes(ts, v) VALUES(?,?)`)
	if err != nil {
		return err
	}
	defer st.Close()
	for i, v := range vals {
		if _, err := st.Exec(startUnix+int64(i)*60, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReplaceBinanceMinutes заменяет весь ряд (vals[i] — минута startUnix+i*60).
func (s *Storage) ReplaceBinanceMinutes(startUnix int64, vals []float64) error {
	return s.putMinutes(startUnix, vals, true)
}

// AppendBinanceMinutes дописывает/перезаписывает точки ряда.
func (s *Storage) AppendBinanceMinutes(startUnix int64, vals []float64) error {
	return s.putMinutes(startUnix, vals, false)
}

func (s *Storage) SetBinanceMeta(k, v string) error {
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`INSERT INTO binance_meta(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, k, v)
	return err
}

func (s *Storage) BinanceMeta(k string) string {
	var v string
	_ = s.coreDB.QueryRow(`SELECT v FROM binance_meta WHERE k=?`, k).Scan(&v)
	return v
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
	var pv sql.NullFloat64
	if err := s.coreDB.QueryRow(`SELECT v FROM binance_minutes WHERE ts < ? ORDER BY ts DESC LIMIT 1`, starts[0]).Scan(&pv); err == nil && pv.Valid {
		prev, havePrev = pv.Float64, true
	}
	rows, err := s.coreDB.Query(`SELECT ts, v FROM binance_minutes WHERE ts >= ? AND ts < ? ORDER BY ts`, starts[0], end)
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

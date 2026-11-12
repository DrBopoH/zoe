package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"economic/pkg/mono"
)

// Binance — такой же источник, как банки: выписка (CSV) → счёт и журнал в КЕШЕ (cache.db, одноразовый).
// Счёт «Binance» — обычная строка accounts (type=binance), его идентификатор BN<id пользователя> кеш знает из выписки
// (в iban-поле списка счетов), поэтому алиас по имени файла получает его сам и попадает в бесхоз; к лицу алиас привязывается
// как любой другой (core.db → alias_accounts).
// Оценка портфеля по минутам — binance_minutes, разбор по монетам для модалки — binance_state. В core.db от Binance нет ничего.

const binanceCacheSchema = `
CREATE TABLE IF NOT EXISTS binance_minutes (
	ts INTEGER PRIMARY KEY,          -- unix-секунды, начало минуты
	v REAL NOT NULL                  -- оценка портфеля, ₴
);
CREATE TABLE IF NOT EXISTS binance_state (k TEXT PRIMARY KEY, v TEXT NOT NULL);
-- журнал выписки целиком: каждая строка как есть; id = время|счёт|операция|монета|сумма|№ повтора
CREATE TABLE IF NOT EXISTS binance_ledger (
	id TEXT PRIMARY KEY,
	t INTEGER NOT NULL,              -- мс
	account TEXT NOT NULL,           -- Spot / Funding / Strategy / USD-M Futures
	op TEXT NOT NULL,
	asset TEXT NOT NULL,
	delta REAL NOT NULL,
	kind TEXT NOT NULL DEFAULT '',   -- внешнее движение (binance.Kind), у внутренних пусто
	note TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_binance_ledger_t ON binance_ledger(t);
-- старая схема: движения собирались из API по периодам
DROP TABLE IF EXISTS binance_flows;
DROP TABLE IF EXISTS binance_coverage;
`

// Виды движений Binance, от которых зависит учёт (те же строки, что в binance.Kind*; db от пакета binance не зависит).
const (
	KindDividendBN = "dividend"
	KindBonusBN    = "bonus"
)

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

// BinanceRow — строка журнала выписки.
type BinanceRow struct {
	T       int64
	Account string
	Op      string
	Asset   string
	Delta   float64
	Kind    string
	Note    string
}

func rowID(r BinanceRow, occ int) string {
	return fmt.Sprintf("%d|%s|%s|%s|%.12g|%d", r.T, r.Account, r.Op, r.Asset, r.Delta, occ)
}

// SaveBinanceStatement кладёт в кеш счёт (имя = имя файла выписки) и все строки журнала. Идемпотентно: одну и ту же строку
// второй раз не пишет, поэтому перекрывающиеся выписки (за разные периоды) просто дополняют друг друга.
// profile — идентификатор счёта (BN<id>): по нему пользователь привязывает счёт к алиасу файла.
func (s *Storage) SaveBinanceStatement(filename, profile string, rows []BinanceRow) error {
	if err := s.saveBinanceStatement(filename, profile, rows); err != nil {
		return err
	}
	s.syncAliases(nil) // профиль появился только сейчас: привязанным к лицу алиасам он попадает в статику (как IBAN банка)
	return nil
}

func (s *Storage) saveBinanceStatement(filename, profile string, rows []BinanceRow) error {
	if err := s.SaveImportedAccount(mono.AccountDTO{ID: BinanceAccountID, Type: BinanceAccountType, CurrencyCode: 980}, filename); err != nil {
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
	seen := map[string]int{}
	var firstNew int64 // самая ранняя из действительно новых строк (мс); 0 — новых нет
	for _, r := range rows {
		base := rowID(r, 0)
		occ := seen[base]
		seen[base]++
		res, err := tx.Exec(`INSERT OR IGNORE INTO binance_ledger(id,t,account,op,asset,delta,kind,note) VALUES(?,?,?,?,?,?,?,?)`,
			rowID(r, occ), r.T, r.Account, r.Op, r.Asset, r.Delta, r.Kind, r.Note)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 && (firstNew == 0 || r.T < firstNew) {
			firstNew = r.T
		}
	}
	if _, err := tx.Exec(`INSERT INTO binance_state(k,v) VALUES('profile',?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, profile); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO binance_state(k,v) VALUES('model',?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, binanceModel); err != nil {
		return err // сброс кеша стирает версию, а журнал после него снова пишется по текущим правилам
	}
	if _, err := tx.Exec(`UPDATE accounts SET opened_at = COALESCE((SELECT MIN(t)/1000 FROM binance_ledger), 0) WHERE id = ?`, BinanceAccountID); err != nil {
		return err
	}
	if err := trimSeriesFrom(tx, firstNew); err != nil {
		return err
	}
	return tx.Commit()
}

// trimSeriesFrom: минутный ряд зависит только от журнала до своей минуты, поэтому новые строки портят его только от самой
// ранней из них. Всё раньше остаётся, обрезается хвост — «Разведать» дорисует его, а не соберёт ряд с нуля.
// Строк новых нет (выписку загрузили повторно) — ряд не трогаем. Новая строка раньше начала ряда — ряд собирается заново.
func trimSeriesFrom(tx *sql.Tx, firstNewMs int64) error {
	if firstNewMs == 0 {
		return nil
	}
	cut := firstNewMs / 60_000 * 60 // минута, в которой лежит новая строка (unix, сек)
	var first sql.NullInt64
	if err := tx.QueryRow(`SELECT MIN(ts) FROM binance_minutes`).Scan(&first); err != nil {
		return err
	}
	if !first.Valid {
		return nil
	}
	if cut <= first.Int64 {
		if _, err := tx.Exec(`DELETE FROM binance_minutes`); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM binance_state WHERE k = 'origin'`) // «оценка до начала ряда» теряет смысл вместе с рядом
		return err
	}
	_, err := tx.Exec(`DELETE FROM binance_minutes WHERE ts >= ?`, cut)
	return err
}

// BinanceLedger — весь журнал по времени.
func (s *Storage) BinanceLedger() ([]BinanceRow, error) {
	rows, err := s.cacheDB.Query(`SELECT t, account, op, asset, delta, kind, note FROM binance_ledger ORDER BY t, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BinanceRow
	for rows.Next() {
		var r BinanceRow
		if err := rows.Scan(&r.T, &r.Account, &r.Op, &r.Asset, &r.Delta, &r.Kind, &r.Note); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// HasBinance — в кеше есть журнал выписки Binance.
func (s *Storage) HasBinance() bool {
	var n int
	_ = s.cacheDB.QueryRow(`SELECT COUNT(*) FROM binance_ledger`).Scan(&n)
	return n > 0
}

// BinanceProfile — идентификатор счёта из последней выписки ("" — выписок не было): его пользователь привязывает к алиасу файла.
func (s *Storage) BinanceProfile() string {
	var v string
	_ = s.cacheDB.QueryRow(`SELECT v FROM binance_state WHERE k='profile'`).Scan(&v)
	return v
}

// BinanceSave — результат оценки портфеля для записи в кеш.
type BinanceSave struct {
	Portfolio float64   // оценка портфеля сейчас, ₴
	Start     int64     // unix-секунды первой минуты Vals
	Vals      []float64 // точки минутного ряда
	Replace   bool      // заменить весь ряд (полная история); иначе дописать точки
	Origin    float64   // оценка «до первой точки» (0 — счёт начался с первого движения); хранится при Replace
	Detail    any
}

// SaveBinance одной транзакцией кладёт в кеш оценку счёта, точки минутного ряда и JSON-разбор по монетам.
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
	if _, err := tx.Exec(`UPDATE accounts SET balance = ?, closed_at = 0 WHERE id = ?`, round2(in.Portfolio), BinanceAccountID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO binance_state(k,v) VALUES('detail',?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, string(raw)); err != nil {
		return err
	}
	return tx.Commit()
}

// BinanceFlowRow — внешнее движение вместе с его id в журнале.
type BinanceFlowRow struct {
	ID    string
	T     int64
	Asset string
	Delta float64
	Kind  string
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
	rows, err := s.cacheDB.Query(`SELECT f.id, f.t, f.asset, f.delta, f.kind FROM binance_ledger f
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

// DropBinanceSeries сбрасывает минутный ряд оценки (журнал остаётся): следующий «Разведать» соберёт его заново.
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

// withBinanceProfile — у счёта Binance идентификатор (BN<id>) известен только из выписки: подставляем его на чтении,
// чтобы его видели и экран, и «кеш → статика» (привязка алиаса к лицу запоминает его в core, сброс кеша его не стирает).
func (s *Storage) withBinanceProfile(list []Account) []Account {
	for i := range list {
		if list[i].Type == BinanceAccountType {
			list[i].IBAN = s.BinanceProfile()
		}
	}
	return list
}

// binanceModel — версия правил учёта. При смене старый ряд и операции, посчитанные по прежним правилам, выбрасываются:
// журнал остаётся, «Разведать» соберёт всё заново. Версия 2: Strategy и Futures — кошельки портфеля, переводы между кошельками внутренние.
const binanceModel = "2"

func (s *Storage) migrateBinanceModel() error {
	var v string
	_ = s.cacheDB.QueryRow(`SELECT v FROM binance_state WHERE k='model'`).Scan(&v)
	if v == binanceModel {
		return nil
	}
	for _, q := range []string{
		`DELETE FROM binance_minutes`,
		`DELETE FROM binance_state WHERE k IN ('origin','detail')`,
		`DELETE FROM transactions WHERE id LIKE 'binance\_%' ESCAPE '\'`,
	} {
		if _, err := s.cacheDB.Exec(q); err != nil {
			return err
		}
	}
	_, err := s.cacheDB.Exec(`INSERT INTO binance_state(k,v) VALUES('model',?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, binanceModel)
	return err
}

type binanceMeta struct{ Kind, Wallet string }

// binanceMetas — вид движения и кошелёк (Spot, Funding, Strategy, USD-M Futures) по id операции счёта ("binance_"+id строки журнала).
func (s *Storage) binanceMetas() map[string]binanceMeta {
	out := map[string]binanceMeta{}
	rows, err := s.cacheDB.Query(`SELECT 'binance_' || id, kind, account FROM binance_ledger WHERE kind != ''`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var m binanceMeta
		if rows.Scan(&id, &m.Kind, &m.Wallet) == nil {
			out[id] = m
		}
	}
	return out
}

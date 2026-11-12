package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"economic/pkg/binance"
	"economic/pkg/db"
)

// Binance — такой же источник, как банки: выписка (CSV) загружается общим импортом (importSource), всё полученное — кеш
// (см. db/binance.go). «Разведать» ничего не качает по счёту: журнал уже в кеше, а оценка портфеля по минутам
// строится из него по публичным минутным ценам (ключей Binance нет). «Вайп кеша» сносит и Binance: после «Загрузить»
// выписка возвращает журнал, следующий «Разведать» — оценку.

type binanceState struct {
	client *binance.Client
}

// SetBinance подключает клиент публичных цен Binance (nil — оценка выключена, журнал всё равно читается).
func (s *Server) SetBinance(c *binance.Client) { s.bn.client = c }

// secs — моменты в мс → в секунды, не чаще раза в час (клеткам хватает; в кеше это короче).
func secs(ms []int64) []int64 {
	var out []int64
	var last int64 = -1
	for _, m := range ms {
		if h := m / 3_600_000; h != last {
			out, last = append(out, m/1000), h
		}
	}
	return out
}

// crawlBinance — Binance в «Разведать»: одна строка «Портфель» в панели прогресса.
func (s *Server) crawlBinance(ctx context.Context) {
	s.bn.client.Stop = func() bool { return ctx.Err() != nil }
	cp := s.prog.client("Binance", "binance", 0)
	started, req0 := time.Now(), s.bn.client.Requests()
	row := &accProg{ID: "binance", Label: "Портфель", State: "fetch", Custom: true, Note: "Оценка по минутным ценам"}
	if rows, err := s.storage.BinanceLedger(); err == nil && len(rows) > 0 {
		row.AxisFrom = rows[0].T / 1000
		var at []int64
		for _, r := range rows {
			at = append(at, r.T)
		}
		row.FlowAt = secs(at)
	}
	s.prog.setAccounts(cp, []*accProg{row})

	s.bn.client.OnWait = func(d time.Duration) { // лимит Binance: «остывание», как у банков
		if d > 0 {
			s.updateStatus(fmt.Sprintf("[Binance] ЛИМИТ ЗАПРОСОВ · ОХЛАЖДЕНИЕ %d С", int(d.Seconds())))
		}
		s.prog.with(row, func(a *accProg) {
			switch {
			case d > 0 && a.State == "fetch":
				a.State, a.CoolUntil = "cooling", time.Now().Add(d).Unix()
			case d == 0 && a.State == "cooling":
				a.State, a.CoolUntil = "fetch", 0
			}
		})
	}
	err := s.syncBinance(ctx, func(done, total int) {
		if total == 0 {
			return
		}
		if n := s.bn.client.Requests() - req0; n >= 3 { // средняя длительность запроса — для «закончится ≈»
			s.prog.mu.Lock()
			cp.ReqSec = time.Since(started).Seconds() / float64(n)
			s.prog.mu.Unlock()
		}
		s.prog.with(row, func(a *accProg) {
			a.Pct, a.Note, a.Left = done*100/total, fmt.Sprintf("запросов %d/%d", done, total), total-done
		})
	})
	if ctx.Err() != nil { // кеш сбросили: панель и статус уже сняты, дописывать нечего
		return
	}
	log.Printf("[Sync] Binance: запросов %d за %s", s.bn.client.Requests()-req0, time.Since(started).Round(time.Second))
	s.prog.with(row, func(a *accProg) {
		a.Note, a.Win, a.CoolUntil, a.Left = "", nil, 0, 0
		if err != nil {
			a.State, a.Error = "error", err.Error()
		} else {
			a.State, a.Pct = "done", 100
		}
	})
	if err != nil {
		log.Printf("[Sync Error] Binance: %v", err)
		s.updateStatus("[Binance] СБОЙ: " + err.Error())
	}
}

// syncBinance: журнал из кеша → остатки → оценка в ₴ → минутный ряд (с нуля, если в кеше его нет; иначе дописать до сейчас) → кеш.
func (s *Server) syncBinance(ctx context.Context, progress func(done, total int)) error {
	c := s.bn.client
	if c == nil {
		return errors.New("Binance: клиент цен не подключён")
	}
	ledger, err := s.storage.BinanceLedger()
	if err != nil {
		return err
	}
	if len(ledger) == 0 {
		return nil // выписку ещё не загружали
	}
	rows := make([]binance.Row, len(ledger))
	for i, r := range ledger {
		rows[i] = binance.Row{T: r.T, Account: r.Account, Op: r.Op, Asset: r.Asset, Delta: r.Delta, Note: r.Note}
	}
	h := binance.HoldingsOf(rows)
	prices, err := c.Prices()
	if err != nil {
		return err
	}
	v, ok := binance.Value(h, prices)
	if !ok {
		return errors.New("нет курса USDT/UAH — оценить портфель в гривнах нечем")
	}
	s.valueBinanceTxs(ctx, c) // операции счёта с оценкой в ₴ (внешние движения журнала)

	now := time.Now()
	flows := binance.Flows(rows)
	var flowAt []int64
	for _, f := range flows {
		flowAt = append(flowAt, f.T)
	}
	var warns []string
	var (
		start   int64
		vals    []float64
		replace bool
	)
	totals := map[string]float64{}
	for a, w := range h {
		if t := w.Total(); t != 0 {
			totals[a] = t
		}
	}
	if last, _, have := s.storage.BinanceLastMinute(); have {
		res, err := c.ExtendSeries(totals, last*1000, now.UnixMilli())
		if err != nil {
			warns = append(warns, "минутный ряд: "+err.Error())
		} else if len(res.Values) > 0 {
			start, vals = res.StartMs/1000, res.Values
		}
	} else if len(flows) > 0 {
		syms, err := c.Symbols()
		if err != nil {
			return err
		}
		pts := binance.Timeline(flows, now.UnixMilli())
		g0 := pts[0].T / 60_000 * 60_000
		res, err := c.Series(pts, g0, now.UnixMilli()/60_000*60_000, syms, progress)
		if err != nil {
			return err
		}
		start, vals, replace = res.StartMs/1000, res.Values, true
		if len(res.Unpriced) > 0 {
			warns = append(warns, "нет минутных цен: "+strings.Join(res.Unpriced, ", "))
		}
	}
	if ctx.Err() != nil { // кеш сбросили, пока считали: в чистую базу не пишем
		return binance.ErrStopped
	}
	detail := map[string]any{
		"taken_at": now.Unix(), "total": v.TotalUAH, "usdt_uah": v.USDTUAH, "items": v.Items, "unpriced": v.Unpriced,
		"history":  map[string]any{"flows": len(flows), "at": now.Unix(), "flow_at": secs(flowAt)},
		"warnings": warns,
	}
	// Баланс счёта = последняя точка минутного ряда, а не оценка по «последней цене»: график опирается на этот баланс как на
	// конец ряда, и любая разница (цены двух запросов в разные секунды) сдвигала бы всю остальную часть капитала.
	portfolio := v.TotalUAH
	if n := len(vals); n > 0 {
		portfolio = vals[n-1]
	} else if _, lv, ok := s.storage.BinanceLastMinute(); ok {
		portfolio = lv
	}
	if err := s.storage.SaveBinance(db.BinanceSave{Portfolio: portfolio, Start: start, Vals: vals, Replace: replace, Detail: detail}); err != nil {
		return err
	}
	for _, w := range warns {
		log.Printf("[Sync] Binance: %s", w)
	}
	first, last, n := s.storage.BinanceSeriesInfo()
	log.Printf("[Sync] Binance: портфель %.2f ₴, ряд %s — %s (%d мин), %s", v.TotalUAH,
		time.Unix(first, 0).Format("2006-01-02"), time.Unix(last, 0).Format("2006-01-02 15:04"), n, map[bool]string{true: "собран с нуля", false: "дописан"}[replace])
	return nil
}

// valueBinanceTxs оценивает в ₴ внешние движения, которых ещё нет среди операций счёта, и сразу кладёт их в кеш.
func (s *Server) valueBinanceTxs(ctx context.Context, c *binance.Client) {
	rows, err := s.storage.BinanceUnvaluedFlows(binance.ExtKinds())
	if err != nil {
		log.Printf("[Sync Error] Binance: операции счёта: %v", err)
		return
	}
	var txs []db.BinanceTx
	for _, r := range rows {
		lbl, _ := binance.ExtLabel(r.Kind)
		uah, err := c.ValueUAH(r.Asset, r.Delta, r.T)
		if err != nil {
			log.Printf("[Sync] Binance: %s %s — оценить в ₴ не вышло: %v", lbl, r.Asset, err)
		}
		qty := strconv.FormatFloat(math.Abs(r.Delta), 'f', -1, 64)
		txs = append(txs, db.BinanceTx{FlowID: r.ID, T: r.T, Desc: fmt.Sprintf("Binance: %s %s %s", lbl, qty, r.Asset), UAH: uah})
	}
	if ctx.Err() != nil {
		return
	}
	if err := s.storage.SaveBinanceTxs(txs); err != nil {
		log.Printf("[Sync Error] Binance: операции счёта: %v", err)
	}
}

// importBinanceStatement — выписка Binance в общем импорте («Дебаг → Импорт», «Загрузить»): счёт (имя = имя файла) и весь журнал
// в кеш; привязка к лицу — алиас по имени файла, как у любого счёта. Оценку портфеля соберёт следующий «Разведать».
func (s *Server) importBinanceStatement(filename string, raw []byte) (*importResult, error) {
	st, err := binance.ParseStatement(bytes.NewReader(raw), binance.StatementZone(filename))
	if err != nil {
		return nil, &importError{http.StatusBadRequest, "Failed to parse Binance statement: " + err.Error()}
	}
	rows := make([]db.BinanceRow, len(st.Rows))
	for i, r := range st.Rows {
		rows[i] = db.BinanceRow{T: r.T, Account: r.Account, Op: r.Op, Asset: r.Asset, Delta: r.Delta, Kind: binance.Kind(r), Note: r.Note}
	}
	if err := s.storage.SaveBinanceStatement(filename, binance.ProfileID(st.UserID), rows); err != nil {
		return nil, &importError{http.StatusInternalServerError, "Failed to save Binance statement: " + err.Error()}
	}
	return &importResult{AccountID: db.BinanceAccountID, Card: "binance", Count: len(rows)}, nil
}

// GET /api/binance — что запомнил кеш: разбор по монетам и пределы минутного ряда. Ничего не опрашивает.
func (s *Server) HandleBinance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	first, last, n := s.storage.BinanceSeriesInfo()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"loaded": s.storage.HasBinance(), "profile": s.storage.BinanceProfile(), "detail": s.storage.BinanceDetail(),
		"first": first, "last": last, "points": n,
	})
}

// POST /api/binance/candles {starts:[unix…], end:unix} → has/first/last/last_value и свечи оценки портфеля по корзинам [starts[i], starts[i+1]).
func (s *Server) HandleBinanceCandles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Starts []int64 `json:"starts"`
		End    int64   `json:"end"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out := map[string]any{"has": false}
	if first, last, n := s.storage.BinanceSeriesInfo(); n > 0 {
		_, lv, _ := s.storage.BinanceLastMinute()
		out = map[string]any{"has": true, "first": first, "last": last, "last_value": lv}
		if len(in.Starts) > 0 {
			if in.End <= in.Starts[len(in.Starts)-1] {
				in.End = in.Starts[len(in.Starts)-1] + 1
			}
			c, has, err := s.storage.BinanceCandles(in.Starts, in.End)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			list := make([]any, len(c))
			for i := range c {
				if has[i] {
					list[i] = c[i]
				}
			}
			out["candles"] = list
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// bnRows — журнал выписки в виде строк пакета binance.
func (s *Server) bnRows() ([]binance.Row, error) {
	ledger, err := s.storage.BinanceLedger()
	if err != nil {
		return nil, err
	}
	rows := make([]binance.Row, len(ledger))
	for i, r := range ledger {
		rows[i] = binance.Row{T: r.T, Account: r.Account, Op: r.Op, Asset: r.Asset, Delta: r.Delta, Note: r.Note}
	}
	return rows, nil
}

// GET /api/binance/moves — переводы между кошельками (Spot ↔ Funding/Strategy/Futures/Earn) из журнала: строки истории для «Операционной».
func (s *Server) HandleBinanceMoves(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := s.bnRows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	moves := binance.Moves(rows)
	if moves == nil {
		moves = []binance.Move{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(moves)
}

// tradeWallets — кошельки, чьи сделки показываем в истории.
var tradeWallets = []string{binance.WalletSpot}

// GET /api/binance/trades — покупки и продажи монет внутри кошельков (сейчас Spot): строки истории, капитал не меняют.
func (s *Server) HandleBinanceTrades(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := s.bnRows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	trades := binance.Trades(rows, tradeWallets...)
	if trades == nil {
		trades = []binance.Trade{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(trades)
}

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"economic/pkg/binance"
)

// Binance: read-only ключ из .env (BINANCE_API_KEY / BINANCE_API_SECRET). Оценка портфеля в ₴ снимается раз в N минут,
// по кнопке «Обновить» и при полном синке; каждый день хранится последний снимок (см. db/binance.go).

type binanceState struct {
	mu      sync.Mutex // один запрос к Binance за раз
	client  *binance.Client
	lastErr string
	warns   []string

	hmu      sync.Mutex // состояние восстановления истории
	hist     histStatus
	histFrom time.Time
}

// histStatus — ход восстановления истории (минутный ряд оценки портфеля).
type histStatus struct {
	Running  bool               `json:"running"`
	Stage    string             `json:"stage,omitempty"`
	Done     int                `json:"done"`
	Total    int                `json:"total"`
	Error    string             `json:"error,omitempty"`
	Flows    int                `json:"flows"`
	Warnings []string           `json:"warnings,omitempty"`
	Residual map[string]float64 `json:"residual,omitempty"` // монета → то, что API не объяснил
	At       int64              `json:"at,omitempty"`       // когда восстановили (unix)
	tried    bool
}

// histStages — порядок этапов восстановления (так их называет binance.BuildHistory); по нему считается общий процент.
var histStages = []string{"остатки", "депозиты", "выводы", "конвертации", "покупки картой", "P2P", "начисления", "сделки", "цены"}

func (h histStatus) percent() int {
	for i, st := range histStages {
		if st == h.Stage {
			frac := 0.0
			if h.Total > 0 {
				frac = float64(h.Done) / float64(h.Total)
			}
			return int((float64(i) + frac) * 100 / float64(len(histStages)))
		}
	}
	return 0
}

func (h histStatus) note() string {
	if h.Total > 1 {
		return fmt.Sprintf("%s · %d/%d", h.Stage, h.Done, h.Total)
	}
	return h.Stage
}

// crawlBinance — участие Binance в «Разведать»: клиент в панели прогресса с двумя «счетами».
// Портфель — оценка остатков (быстро). История по минутам — если ещё не восстановлена (или идёт), ждём её конца здесь же.
func (s *Server) crawlBinance() {
	cp := s.prog.client("Binance", "binance", 0)
	port := &accProg{ID: "binance", Label: "Портфель", State: "fetch", Custom: true, Note: "остатки и оценка в ₴"}
	s.prog.setAccounts(cp, []*accProg{port})
	if err := s.SyncBinance(); err != nil {
		log.Printf("WARN: Binance: %v", err)
		s.prog.with(port, func(a *accProg) { a.State, a.Error = "error", err.Error() })
		return
	}
	s.prog.with(port, func(a *accProg) { a.State, a.Note = "done", "" })

	// история: уже восстановлена — ряд дописался в SyncBinance; нет — запускаем (если не идёт) и показываем ход
	b := &s.bn
	b.hmu.Lock()
	running := b.hist.Running
	b.hmu.Unlock()
	if !running && s.storage.BinanceMeta("backfill_at") != "" {
		return
	}
	if !running {
		if err := s.StartBinanceHistory(); err != nil {
			return
		}
	}
	his := &accProg{ID: "binance-history", Label: "История по минутам", State: "fetch", Custom: true}
	s.prog.setAccounts(cp, []*accProg{port, his})
	for {
		b.hmu.Lock()
		h := b.hist
		b.hmu.Unlock()
		s.prog.with(his, func(a *accProg) { a.Pct, a.Note = h.percent(), h.note() })
		if !h.Running {
			s.prog.with(his, func(a *accProg) {
				a.Note = ""
				if h.Error != "" {
					a.State, a.Error = "error", h.Error
				} else {
					a.State = "done"
				}
			})
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// SetBinanceHistoryFrom — как далеко назад искать движения (по умолчанию 2020-01-01).
func (s *Server) SetBinanceHistoryFrom(t time.Time) { s.bn.histFrom = t }

// StartBinanceHistory запускает восстановление истории в фоне (одно за раз).
func (s *Server) StartBinanceHistory() error {
	b := &s.bn
	if b.client == nil {
		return errors.New("Binance не подключён")
	}
	b.hmu.Lock()
	if b.hist.Running {
		b.hmu.Unlock()
		return errors.New("восстановление уже идёт")
	}
	b.hist = histStatus{Running: true, Stage: "старт", tried: true}
	b.hmu.Unlock()
	go s.runBinanceHistory()
	return nil
}

func (s *Server) runBinanceHistory() {
	b := &s.bn
	set := func(f func(h *histStatus)) {
		b.hmu.Lock()
		f(&b.hist)
		b.hmu.Unlock()
	}
	fail := func(err error) {
		log.Printf("WARN: Binance история: %v", err)
		set(func(h *histStatus) { h.Running, h.Error = false, err.Error() })
	}
	from := b.histFrom
	if from.IsZero() {
		from = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	res, err := b.client.BuildHistory(binance.HistoryOpts{From: from, Progress: func(stage string, d, t int) {
		set(func(h *histStatus) { h.Stage, h.Done, h.Total = stage, d, t })
	}})
	if err != nil {
		fail(err)
		return
	}
	if res.Series == nil || len(res.Series.Values) == 0 {
		set(func(h *histStatus) {
			h.Running, h.Warnings, h.Error = false, res.Warnings, "движений на счёте не нашлось — восстанавливать нечего"
		})
		return
	}
	if err := s.storage.ReplaceBinanceMinutes(res.Series.StartMs/1000, res.Series.Values); err != nil {
		fail(err)
		return
	}
	now := time.Now().Unix()
	_ = s.storage.SetBinanceMeta("backfill_at", strconv.FormatInt(now, 10))
	set(func(h *histStatus) {
		*h = histStatus{Stage: "готово", Flows: res.Flows, Warnings: res.Warnings, Residual: res.Residual, At: now, tried: true}
	})
	b.hmu.Lock()
	raw, err := json.Marshal(b.hist)
	b.hmu.Unlock()
	if err == nil {
		_ = s.storage.SetBinanceMeta("history", string(raw))
	}
}

// historyView — состояние для интерфейса: ход текущего восстановления или итог прошлого.
func (s *Server) historyView() map[string]any {
	b := &s.bn
	b.hmu.Lock()
	h := b.hist
	b.hmu.Unlock()
	if !h.Running && h.At == 0 && h.Error == "" {
		if raw := s.storage.BinanceMeta("history"); raw != "" {
			_ = json.Unmarshal([]byte(raw), &h)
		}
	}
	first, last, n := s.storage.BinanceSeriesInfo()
	return map[string]any{"status": h, "first": first, "last": last, "points": n}
}

// extendBinanceSeries дописывает минутный ряд до текущего момента (если история уже восстановлена).
func (s *Server) extendBinanceSeries(h binance.Holdings) error {
	if s.storage.BinanceMeta("backfill_at") == "" {
		return nil
	}
	s.bn.hmu.Lock()
	running := s.bn.hist.Running
	s.bn.hmu.Unlock()
	if running {
		return nil
	}
	last, _, ok := s.storage.BinanceLastMinute()
	if !ok {
		return nil
	}
	res, err := s.bn.client.ExtendSeries(h, last*1000, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	if len(res.Values) == 0 {
		return nil
	}
	return s.storage.AppendBinanceMinutes(res.StartMs/1000, res.Values)
}

// SetBinance подключает клиент Binance (nil — интеграция выключена).
func (s *Server) SetBinance(c *binance.Client) { s.bn.client = c }

// SyncBinance снимает остатки, оценивает в гривнах и сохраняет снимок дня.
func (s *Server) SyncBinance() error {
	b := &s.bn
	if b.client == nil {
		return errors.New("Binance не подключён: нет BINANCE_API_KEY / BINANCE_API_SECRET")
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	err := s.syncBinanceLocked()
	if err != nil {
		b.lastErr = err.Error()
		return err
	}
	b.lastErr = ""
	return nil
}

func (s *Server) syncBinanceLocked() error {
	b := &s.bn
	h, warns, err := b.client.Fetch()
	if err != nil {
		return err
	}
	prices, err := b.client.Prices()
	if err != nil {
		return err
	}
	v, ok := binance.Value(h, prices)
	if !ok {
		return errors.New("нет курса USDT/UAH — оценить портфель в гривнах нечем")
	}
	if err := s.extendBinanceSeries(h); err != nil {
		warns = append(warns, "минутный ряд: "+err.Error())
	}
	b.warns = warns
	detail := map[string]any{"items": v.Items, "unpriced": v.Unpriced, "warnings": warns}
	return s.storage.SaveBinanceSnapshot(v.TotalUAH, v.USDTUAH, detail, time.Now())
}

// RunBinanceLoop опрашивает Binance сразу и затем каждые every, пока жив ctx.
func (s *Server) RunBinanceLoop(ctx context.Context, every time.Duration) {
	if s.bn.client == nil {
		return
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			if err := s.SyncBinance(); err != nil {
				log.Printf("WARN: Binance: %v", err)
			} else if s.storage.BinanceMeta("backfill_at") == "" {
				// первый успешный опрос: восстанавливаем прошлое (один раз за запуск, пока оно не получилось)
				s.bn.hmu.Lock()
				tried := s.bn.hist.tried
				s.bn.hmu.Unlock()
				if !tried {
					_ = s.StartBinanceHistory()
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// GET /api/binance — последний снимок и состояние интеграции; POST — снять сейчас и вернуть то же.
func (s *Server) HandleBinance(w http.ResponseWriter, r *http.Request) {
	var syncErr string
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		if err := s.SyncBinance(); err != nil {
			syncErr = err.Error()
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ov, err := s.storage.BinanceOverview(s.bn.client != nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.bn.mu.Lock()
	lastErr := s.bn.lastErr
	s.bn.mu.Unlock()
	if syncErr != "" {
		lastErr = syncErr
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"configured": ov.Configured, "snapshot": ov.Snapshot, "days": ov.Days, "error": lastErr, "history": s.historyView(),
	})
}

// GET /api/binance/history — ход/итог восстановления; POST — запустить (заново).
func (s *Server) HandleBinanceHistory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		if err := s.StartBinanceHistory(); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.historyView())
}

// POST /api/binance/candles {starts:[unix…], end:unix} → has/first/last/last_value и свечи оценки портфеля по корзинам [starts[i], starts[i+1]).
// has=false — минутного ряда нет (график считает Binance как раньше, по дневным снимкам).
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

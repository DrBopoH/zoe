package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"economic/pkg/market"
)

// Рыночные данные по облигациям (ОВДП НБУ, US Treasury) — отдельный процесс рядом с обходом банков: свой статус,
// свои итоги, свои тосты. Запускается вместе с «Разведать» (HandleFullHistoricalSync) или сам (POST /api/market/sync),
// источники-«фиды» идут параллельно и не знают друг о друге. Публичные API без ключей, кеш стирается вайпом.

// SetMarket подключает клиент публичных облигационных данных (nil — обновление выключено, кеш читается).
func (s *Server) SetMarket(c *market.Client) { s.mk.client = c }

const marketFeedTimeout = 90 * time.Second

type feedState struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	State   string `json:"state"`          // queued | fetch | done | error
	Pct     int    `json:"pct,omitempty"`  // ход долгого источника, 0 — неизвестен (бегущая полоса)
	Note    string `json:"note,omitempty"` // что идёт сейчас («аукционы: 10.11»)
	Fetched int    `json:"fetched"`
	Skipped int    `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
}

type marketResult struct {
	FinishedAt int64       `json:"finished_at"`
	Feeds      []feedState `json:"feeds"`
}

type marketState struct {
	client *market.Client

	mu        sync.Mutex
	running   bool
	startedAt int64
	feeds     []*feedState
	last      *marketResult
	wg        sync.WaitGroup // идущий опрос; abortSync ждёт его перед сбросом кеша
}

// marketSnapshot — состояние для /api/sync/status (ключ "market"): то, что рисует фронт.
func (s *Server) marketSnapshot() map[string]any {
	m := &s.mk
	m.mu.Lock()
	defer m.mu.Unlock()
	feeds := make([]feedState, len(m.feeds))
	for i, f := range m.feeds {
		feeds[i] = *f
	}
	return map[string]any{"running": m.running, "started_at": m.startedAt, "feeds": feeds, "last": m.last}
}

// startMarket запускает опрос в фоне. only — один источник (пусто — все). false: клиента нет или опрос уже идёт.
// ctx отменяет опрос (сброс кеша); недописанное после отмены в кеш не попадает.
func (s *Server) startMarket(ctx context.Context, only string) bool {
	m := &s.mk
	if m.client == nil {
		return false
	}
	var feeds []market.Feed
	for _, f := range m.client.Feeds() {
		if only == "" || only == f.ID {
			feeds = append(feeds, f)
		}
	}
	m.mu.Lock()
	if m.running || len(feeds) == 0 {
		m.mu.Unlock()
		return false
	}
	m.running, m.startedAt = true, time.Now().Unix()
	m.feeds = make([]*feedState, len(feeds))
	for i, f := range feeds {
		m.feeds[i] = &feedState{ID: f.ID, Title: f.Title, State: "queued"}
	}
	states := m.feeds
	m.wg.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.wg.Done()
		var wg sync.WaitGroup
		for i, f := range feeds {
			wg.Add(1)
			go func() {
				defer wg.Done()
				st := states[i]
				m.mu.Lock()
				st.State = "fetch"
				m.mu.Unlock()
				fctx, cancel := context.WithTimeout(ctx, marketFeedTimeout)
				defer cancel()
				list, res := f.Fetch(fctx, func(pct int, note string) {
					m.mu.Lock()
					st.Pct, st.Note = pct, note
					m.mu.Unlock()
				})
				if ctx.Err() == nil && len(list) > 0 {
					if err := s.storage.SaveMarketBonds(list, time.Now().Unix()); err != nil {
						res.Error = strings.TrimSpace(res.Error + " сохранение: " + err.Error())
					}
				}
				m.mu.Lock()
				st.Fetched, st.Skipped, st.Error, st.Note = res.Fetched, res.Skipped, res.Error, ""
				if res.Error != "" {
					st.State = "error"
				} else {
					st.State, st.Pct = "done", 100
				}
				m.mu.Unlock()
			}()
		}
		wg.Wait()
		m.mu.Lock()
		defer m.mu.Unlock()
		m.running = false
		if ctx.Err() != nil { // сброс кеша: итог не нужен, панель очищена
			m.feeds, m.last = nil, nil
			return
		}
		res := &marketResult{FinishedAt: time.Now().Unix(), Feeds: make([]feedState, len(m.feeds))}
		for i, f := range m.feeds {
			res.Feeds[i] = *f
		}
		m.last = res
	}()
	return true
}

// waitMarket ждёт окончания идущего опроса, но не дольше d (вызывается после отмены контекста).
func (s *Server) waitMarket(d time.Duration) {
	done := make(chan struct{})
	go func() { s.mk.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// GET /api/market/bonds?source=nbu|ust&q=… — скачанные бумаги (q — часть ISIN/CUSIP или названия).
func (s *Server) HandleMarketBonds(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	src := strings.ToLower(r.URL.Query().Get("source"))
	if src != "" && src != market.SourceNBU && src != market.SourceUST {
		fail(w, http.StatusBadRequest, "source: nbu или ust")
		return
	}
	list, err := s.storage.MarketBonds(src)
	if err != nil {
		failErr(w, err)
		return
	}
	if q := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("q"))); q != "" {
		out := list[:0]
		for _, b := range list {
			if strings.Contains(b.ID, q) || strings.Contains(strings.ToUpper(b.Name), q) {
				out = append(out, b)
			}
		}
		list = out
	}
	writeJSON(w, list)
}

// POST /api/market/sync[?source=nbu|ust] — запустить опрос отдельно от «Разведать». Ответ сразу, ход и итог —
// в "market" из /api/sync/status. Уже идёт — 409.
func (s *Server) HandleMarketSync(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}
	if s.mk.client == nil {
		fail(w, http.StatusServiceUnavailable, "источник облигаций не подключён")
		return
	}
	src := strings.ToLower(r.URL.Query().Get("source"))
	if src != "" && src != market.SourceNBU && src != market.SourceUST {
		fail(w, http.StatusBadRequest, "source: nbu или ust")
		return
	}
	s.syncMutex.Lock()
	if s.marketCtx == nil {
		s.marketCtx, s.marketCancel = context.WithCancel(context.Background())
	}
	ctx := s.marketCtx
	s.syncMutex.Unlock()
	if !s.startMarket(ctx, src) {
		fail(w, http.StatusConflict, "обновление уже идёт")
		return
	}
	writeMsg(w, "started", "Обновление облигаций запущено.")
}

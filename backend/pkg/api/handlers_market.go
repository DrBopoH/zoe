package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"economic/pkg/market"
)

// Рыночные данные по облигациям: ОВДП (НБУ) и US Treasury. Публичные источники, ключей нет; всё лежит в кеше
// (db/market.go) и стирается «Вайпом кеша». Обновляется только по запросу — само ничего не качает.

// SetMarket подключает клиент публичных облигационных данных (nil — обновление выключено, кеш читается).
func (s *Server) SetMarket(c *market.Client) { s.mk.client = c }

type marketState struct {
	client  *market.Client
	syncing chan struct{} // семафор на один идущий опрос; nil до первого использования
}

const marketSyncTimeout = 90 * time.Second

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

// POST /api/market/sync[?source=nbu|ust] — обновить кеш из публичных источников. Источники независимы: упавший
// не мешает другому, ошибка каждого — в его строке ответа. Одновременно идёт только один опрос (иначе 409).
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
	if s.mk.syncing == nil {
		s.mk.syncing = make(chan struct{}, 1)
	}
	sem := s.mk.syncing
	s.syncMutex.Unlock()
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	default:
		fail(w, http.StatusConflict, "обновление уже идёт")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), marketSyncTimeout)
	defer cancel()
	mc := s.mk.client
	var results []market.Result
	run := func(source string, fetch func() ([]market.Security, market.Result)) {
		list, res := fetch()
		if len(list) > 0 {
			if err := s.storage.SaveMarketBonds(list, time.Now().Unix()); err != nil {
				res.Error = strings.TrimSpace(res.Error + " сохранение: " + err.Error())
			}
		}
		results = append(results, res)
	}
	if src == "" || src == market.SourceNBU {
		run(market.SourceNBU, func() ([]market.Security, market.Result) { return mc.SyncNBU(ctx) })
	}
	if src == "" || src == market.SourceUST {
		run(market.SourceUST, func() ([]market.Security, market.Result) {
			list, res, err := mc.USTAuctions(ctx)
			if err != nil {
				res.Error = err.Error()
			}
			return list, res
		})
	}
	writeJSON(w, map[string]any{"results": results})
}

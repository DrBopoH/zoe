package api

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"economic/pkg/db"
	"economic/pkg/market"
	"economic/pkg/source"
)

// Рыночные данные по облигациям (ОВДП НБУ, US Treasury) — отдельный процесс рядом с обходом банков: свой статус,
// свои итоги, свои тосты. Запускается вместе с «Разведать» (HandleFullHistoricalSync) или сам (POST /api/market/sync),
// источники-«фиды» идут параллельно и не знают друг о друге. Публичные API без ключей, кеш стирается вайпом.

// SetMarket подключает клиент публичных облигационных данных (nil — обновление выключено, кеш читается).
func (s *Server) SetMarket(c *market.Client) {
	if c != nil && c.LastDate == nil {
		c.LastDate = s.storage.MacroLastDate // тяжёлые дневные ряды FRED докачиваются хвостом от последней сохранённой даты
	}
	s.mk.client = c
}

const marketFeedTimeout = 3 * time.Minute // с запасом на повторы запросов (market.Client.Retries): источник может отвечать медленно

type feedState struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Unit    string `json:"unit,omitempty"` // что считает источник: «бумаг», «точек»
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
	quiet     bool // фоновый опрос по расписанию: в статусе не виден (без панели и тостов)
	startedAt int64
	feeds     []*feedState
	last      *marketResult
	every     time.Duration  // шаг сетки снимков по расписанию (0 — market.TickEvery)
	wg        sync.WaitGroup // идущий опрос; abortSync ждёт его перед сбросом кеша
}

// marketSnapshot — состояние для /api/sync/status (ключ "market"): то, что рисует фронт.
func (s *Server) marketSnapshot() map[string]any {
	m := &s.mk
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.quiet { // фоновый опрос по расписанию не должен каждый час открывать панель и сыпать тостами
		return map[string]any{"running": false, "started_at": 0, "feeds": []feedState{}, "last": m.last}
	}
	feeds := make([]feedState, len(m.feeds))
	for i, f := range m.feeds {
		feeds[i] = *f
	}
	return map[string]any{"running": m.running, "started_at": m.startedAt, "feeds": feeds, "last": m.last}
}

// startMarket запускает опрос в фоне. only — один источник или несколько через запятую (пусто — все). false: клиента нет или опрос уже идёт.
// ctx отменяет опрос (сброс кеша); недописанное после отмены в кеш не попадает.
// quiet — опрос по расписанию: итог не сохраняется в last и статус его не показывает.
func (s *Server) startMarket(ctx context.Context, only string, quiet bool) bool {
	m := &s.mk
	if m.client == nil {
		return false
	}
	var feeds []market.Feed
	want := market.SplitSources(only)
	for _, f := range m.client.Feeds() {
		if len(want) == 0 || slices.Contains(want, f.ID) {
			feeds = append(feeds, f)
		}
	}
	m.mu.Lock()
	if m.running || len(feeds) == 0 {
		m.mu.Unlock()
		return false
	}
	m.running, m.quiet, m.startedAt = true, quiet, time.Now().Unix()
	m.feeds = make([]*feedState, len(feeds))
	for i, f := range feeds {
		m.feeds[i] = &feedState{ID: f.ID, Title: f.Title, State: "queued", Unit: f.Unit}
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
				rep := func(pct int, note string) {
					m.mu.Lock()
					st.Pct, st.Note = pct, note
					m.mu.Unlock()
				}
				var res market.Result
				if f.Series != nil {
					var pts []market.SeriesPoint
					pts, res = f.Series(fctx, rep)
					if ctx.Err() == nil && len(pts) > 0 {
						now := time.Now().Unix()
						if err := s.storage.SaveMacroSeries(pts, now, res.Tail...); err != nil {
							res.Error = strings.TrimSpace(res.Error + " сохранение: " + err.Error())
						}
						// воспроизводимые ряды источника — в файл группы «Экономика»: из него кеш собирается заново (RestoreEconomy)
						if err := s.persistSeries(f.ID, pts); err != nil {
							res.Error = strings.TrimSpace(res.Error + " файл рядов: " + err.Error())
						}
						// снимки (P2P, комиссии, доминация) заодно копятся по часам в файле группы «Экономика»: заново их не получить
						step := int64(s.tickEvery().Seconds())
						if err := store().AppendTicks(source.SetChainTicks, step, now-now%step, snapshotValues(pts)); err != nil {
							res.Error = strings.TrimSpace(res.Error + " часовые точки: " + err.Error())
						}
					}
				} else if f.Companies != nil {
					var list []market.Company
					list, res = f.Companies(fctx, rep)
					if ctx.Err() == nil && len(list) > 0 {
						if err := s.storage.SaveMarketCompanies(list, time.Now().Unix()); err != nil {
							res.Error = strings.TrimSpace(res.Error + " сохранение: " + err.Error())
						}
					}
				} else {
					var list []market.Security
					list, res = f.Fetch(fctx, rep)
					if ctx.Err() == nil && len(list) > 0 {
						if err := s.storage.SaveMarketBonds(list, time.Now().Unix()); err != nil {
							res.Error = strings.TrimSpace(res.Error + " сохранение: " + err.Error())
						}
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
			m.feeds, m.last, m.quiet = nil, nil, false
			return
		}
		if m.quiet { // расписание: прошлый итог пользовательского опроса остаётся как был
			m.quiet = false
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

// GET /api/market/companies?q=…&limit=N — список компаний для раздела «Акции» (Nasdaq + SEC), крупные по капитализации первыми.
// q — часть тикера или названия, limit — сколько вернуть (0 или нет — все).
func (s *Server) HandleMarketCompanies(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			fail(w, http.StatusBadRequest, "limit: целое число, 0 — все")
			return
		}
		limit = n
	}
	list, err := s.storage.MarketCompanies(r.URL.Query().Get("q"), limit)
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, list)
}

// POST /api/market/sync[?source=<источник>[,<источник>…]; список источников:  nbu|ust|macro|fx|us|usecon|uscurve|usreal|usprices|usjobs|usact|usliq|uscredit|ez|gb|ch|jp|cn|ru|eg|pl|tr|no|se|ca|crypto|stocks|companies] — запустить опрос отдельно от «Разведать». Ответ сразу, ход и итог —
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
	if !market.IsSources(src) {
		fail(w, http.StatusBadRequest, "source: nbu, ust, macro, fx, us, usecon, uscurve, usreal, usprices, usjobs, usact, usliq, uscredit, ez, gb, ch, jp, cn, ru, eg, pl, tr, no, se, ca, commodities, world, chain, ppp, crypto, stocks или companies")
		return
	}
	if !s.startMarket(s.marketContext(), src, false) {
		fail(w, http.StatusConflict, "обновление уже идёт")
		return
	}
	writeMsg(w, "started", "Обновление облигаций запущено.")
}

// /api/market/lists?name=icu — свой список ISIN (хранится в core.db, не в кеше).
//
//	GET            → {"name":"icu","isins":[…],"prices":{"UA…":1117.72},"ytms":{"UA…":16.2}}
//	PUT/POST       — тело {"isins":[…],"prices":{…},"ytms":{…}} заменяет список целиком; цена и доходность необязательны
func (s *Server) HandleMarketLists(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("name")))
	reply := func(ids []string, entries map[string]db.MarketEntry) {
		prices, ytms := map[string]float64{}, map[string]float64{}
		for id, e := range entries {
			if e.Price > 0 {
				prices[id] = e.Price
			}
			if e.YTM > 0 {
				ytms[id] = e.YTM
			}
		}
		writeJSON(w, map[string]any{"name": name, "isins": ids, "prices": prices, "ytms": ytms})
	}
	switch r.Method {
	case http.MethodGet:
		ids, entries, err := s.storage.MarketList(name)
		if err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		reply(ids, entries)
	case http.MethodPut, http.MethodPost:
		var body struct {
			ISINs  []string           `json:"isins"`
			Prices map[string]float64 `json:"prices"`
			YTMs   map[string]float64 `json:"ytms"`
		}
		if !readJSON(w, r, &body) {
			return
		}
		entries := map[string]db.MarketEntry{}
		for id, v := range body.Prices {
			e := entries[strings.ToUpper(strings.TrimSpace(id))]
			e.Price = v
			entries[strings.ToUpper(strings.TrimSpace(id))] = e
		}
		for id, v := range body.YTMs {
			e := entries[strings.ToUpper(strings.TrimSpace(id))]
			e.YTM = v
			entries[strings.ToUpper(strings.TrimSpace(id))] = e
		}
		ids, kept, err := s.storage.SaveMarketList(name, body.ISINs, entries, time.Now().Unix())
		if err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		reply(ids, kept)
	default:
		fail(w, http.StatusMethodNotAllowed, "GET, PUT")
	}
}

// /api/market/curve?name=icu — кривая доходности рынка, заданная пользователем: точки срок (годы) → доходность (%).
//
//	GET            → {"name":"icu","points":[{"years":1,"ytm":15.3}]}
//	PUT/POST       — тело {"points":[…]} заменяет кривую целиком
func (s *Server) HandleMarketCurve(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("name")))
	switch r.Method {
	case http.MethodGet:
		pts, err := s.storage.MarketCurve(name)
		if err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{"name": name, "points": pts})
	case http.MethodPut, http.MethodPost:
		var body struct {
			Points []db.CurvePoint `json:"points"`
		}
		if !readJSON(w, r, &body) {
			return
		}
		pts, err := s.storage.SaveMarketCurve(name, body.Points, time.Now().Unix())
		if err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{"name": name, "points": pts})
	default:
		fail(w, http.StatusMethodNotAllowed, "GET, PUT")
	}
}

// GET /api/macro[?series=policy_rate,cpi_yoy] — макро-ряды из кеша (ставки НБУ, инфляция): {"series":{"policy_rate":[{"d":"2026-01-01","v":15.5}]},"updated_at":…}.
// Ставки — только даты смены значения (действует до следующей точки), инфляция — помесячно. Пусто, пока не было «Разведать».
func (s *Server) HandleMacro(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	var ids []string
	for _, id := range strings.Split(r.URL.Query().Get("series"), ",") {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			ids = append(ids, id)
		}
	}
	series, updated, err := s.storage.MacroSeries(ids)
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"series": series, "updated_at": updated})
}

// GET /api/macro/catalog — какие ряды даёт каждый числовой источник: {"feeds":{"fx":["usd_uah","eur_uah","xau_uah"],…}}.
// Список ведёт бэкенд (market.Feed.Provides): фронт берёт id оттуда, а не хранит свои копии.
// В том же ответе "countries" — реестр стран (market.Countries): какой ряд отвечает за какой показатель карточки страны.
func (s *Server) HandleMacroCatalog(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	feeds := map[string][]string{}
	if c := s.mk.client; c != nil {
		feeds = c.Catalog()
	}
	// snapshots — ряды-снимки: у них есть часовая история (GET /api/macro/ticks), фронт свой список не держит
	writeJSON(w, map[string]any{"feeds": feeds, "countries": market.WithSources(market.Countries(), feeds), "snapshots": market.SnapshotSeries})
}

// GET /api/macro/sources — здоровье источников: что обещает каждый, что лежит в кеше, когда обновлялось, какие ряды устарели, последняя ошибка.
// {"feeds":[{id,title,series,cached,points,last_date,updated_at,stale,error}],"series":[{id,feed,points,from,to,updated_at,freq,lag_days,stale}],"running":false}.
// Ошибка — из последнего опроса этого запуска сервера (после перезапуска её нет, а свежесть считается по кешу).
func (s *Server) HandleMacroSources(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	stats, err := s.storage.MacroStats()
	if err != nil {
		failErr(w, err)
		return
	}
	var feeds []market.Feed
	if c := s.mk.client; c != nil {
		feeds = c.Feeds()
	}
	errs := map[string]string{}
	m := &s.mk
	m.mu.Lock()
	running := m.running
	if m.last != nil {
		for _, f := range m.last.Feeds {
			if f.Error != "" {
				errs[f.ID] = f.Error
			}
		}
	}
	for _, f := range m.feeds {
		if f.Error != "" {
			errs[f.ID] = f.Error
		}
	}
	m.mu.Unlock()
	fh, sh := market.Health(feeds, stats, errs, time.Now())
	if fh == nil {
		fh = []market.FeedHealth{}
	}
	if sh == nil {
		sh = []market.SeriesHealth{}
	}
	writeJSON(w, map[string]any{"feeds": fh, "series": sh, "running": running})
}

// marketContext — общий контекст опросов рынка: его отменяет сброс кеша.
func (s *Server) marketContext() context.Context {
	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()
	if s.marketCtx == nil {
		s.marketCtx, s.marketCancel = context.WithCancel(context.Background())
	}
	return s.marketCtx
}

// tickEvery — шаг сетки часовых снимков.
func (s *Server) tickEvery() time.Duration {
	if s.mk.every > 0 {
		return s.mk.every
	}
	return market.TickEvery
}

// scheduleRetry — пауза после попытки опроса по расписанию: опрос идёт в фоне, снимок появляется через десятки секунд,
// а при сбое (нет сети, опрос занят) следующая попытка — через эту паузу, а не мгновенно.
const scheduleRetry = 2 * time.Minute

// RunMarketSchedule раз в every (по сетке, кратной суткам; 0 — выключено) сам снимает снимки крипто-контура (P2P, комиссии, доминация),
// чтобы у них копилась история без участия пользователя. Привязка к сетке и последнему сохранённому слоту (market.NextTick): рестарт сервера
// не сдвигает расписание, пропущенный час снимается сразу при запуске. Пока идёт опрос пользователя, слот ждёт. Работает до отмены ctx.
func (s *Server) RunMarketSchedule(ctx context.Context, every time.Duration) {
	if every <= 0 {
		return
	}
	s.mk.every = every
	for {
		wait := market.NextTick(time.Now(), time.Unix(store().LastTick(source.SetChainTicks), 0), every)
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			continue // слот мог наступить, а мог сдвинуться: пересчитываем по сетке
		}
		s.startMarket(s.marketContext(), market.SourceChain, true)
		select {
		case <-ctx.Done():
			return
		case <-time.After(scheduleRetry):
		}
	}
}

// GET /api/macro/ticks?series=p2p_ask&since=unix — часовые точки ряда-снимка: [{"t":unix,"v":значение}] по возрастанию времени.
func (s *Server) HandleMacroTicks(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("series"))
	if !market.IsSnapshot(id) {
		fail(w, http.StatusBadRequest, "series: "+strings.Join(market.SnapshotSeries, ", "))
		return
	}
	var since int64
	if v := r.URL.Query().Get("since"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			fail(w, http.StatusBadRequest, "since: unix-время секунд")
			return
		}
		since = n
	}
	writeJSON(w, store().ReadTicks(source.SetChainTicks, id, since))
}

// snapshotValues — значения рядов-снимков из опроса (по последней точке ряда).
func snapshotValues(pts []market.SeriesPoint) map[string]float64 {
	out := map[string]float64{}
	for _, p := range pts {
		if market.IsSnapshot(p.Series) {
			out[p.Series] = p.Value
		}
	}
	return out
}

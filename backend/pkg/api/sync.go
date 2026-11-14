package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"economic/pkg/db"
	"economic/pkg/mono"
)

// errKind — как обход должен реагировать на ошибку банка.
type errKind int

const (
	errFatal      errKind = iota // прервать этот счёт
	errRetry                     // лимит запросов: подождать и повторить то же окно
	errHistoryEnd                // банк сказал «раньше ничего нет»
	errNetwork                   // нет связи с банком (обрыв сети, DNS, таймаут): дальше долбить бессмысленно
)

// errStopped — обход остановлен сбросом кеша: ничего не пишем и не считаем сбоем.
var errStopped = errors.New("обход остановлен")

// errOffline — обход остановлен из-за сети. Оставшиеся счета ключа не опрашиваем и не ждём паузы между запросами.
var errOffline = errors.New("нет связи с банком")

// isNetworkErr — ошибка транспорта (сам запрос не дошёл), а не ответ банка.
func isNetworkErr(err error) bool {
	var ne net.Error
	return errors.As(err, &ne)
}

// walkCfg — всё, что отличает Mono от Raif (и что подменяется в тестах).
type walkCfg struct {
	ctx       context.Context // отмена обхода (сброс кеша); nil — не отменяется
	label     string
	pageLimit int           // сколько операций банк отдаёт максимум за запрос (0 — неизвестно)
	pause     time.Duration // пауза между запросами к банку (для статуса; сам ждёт pace)
	retryWait time.Duration // пауза после «слишком много запросов»
	classify  func(error) errKind
	fetch     func(from, to time.Time) ([]mono.StatementItemDTO, error)
	sleep     func(time.Duration)
	onWindow  func(from, to int64) // необязательный хук: какое окно сейчас запрашивается (для панели прогресса)
	pace      func()               // вызывается перед КАЖДЫМ запросом к банку: выдерживает паузу между запросами одного ключа
	now       func() time.Time
	status    func(string)
}

func (c walkCfg) stopped() bool { return c.ctx != nil && c.ctx.Err() != nil }

const (
	walkWindow     = 30 * 24 * time.Hour
	walkTipMargin  = 6 * time.Hour // свежий край не считаем снятым: банк мог провести операцию с опозданием
	walkMaxEmpty   = 12            // столько пустых окон подряд (≈ год) — считаем, что история закончилась
	walkMaxRetries = 20

	walkNetWait       = 30 * time.Second // пауза перед повтором при обрыве сети
	walkMaxNetRetries = 20               // ≈ 10 минут без связи — сдаёмся и останавливаем ключ
)

type walkStats struct {
	Requests int  // сколько запросов к банку реально сделано
	Complete bool // дошли до начала истории
}

// finish — история счёта снята целиком: фиксируем реальную дату открытия (первая операция выписки).
func (s *Server) finish(accID string, st *walkStats) {
	st.Complete = true
	if err := s.storage.RecordOpenedAt(accID); err != nil {
		log.Printf("[Sync] opened_at %s: %v", accID, err)
	}
}

// walkAccount снимает историю счёта от «сейчас» назад, но только там, где кеш ещё не знает ответа.
//
//	Память — таблица покрытия: какие отрезки времени уже сняты полностью.
//	Отрезок, в который упёрлись, перепрыгиваем без запроса к банку;
//	окно, которое упирается в уже снятое, обрезается по его краю;
//	дошли до отрезка с From == HistoryStart — история целиком в кеше, выходим.
//
// Три сценария из этого получаются сами собой:
//
//	кеш свежий                → 1 запрос (хвост от прошлой синхронизации до сейчас), выход
//	кеш отстал на месяц       → окна только по дыре, стык с кешем склеивается, дальше прыжок до начала
//	прерванная долгая выгрузка → свежий кусок + прыжок через снятое + продолжение с последней дыры
func (s *Server) walkAccount(accID string, cfg walkCfg) (walkStats, error) {
	w := s.newWalker(accID, cfg)
	for {
		finished, err := w.step()
		if err != nil || finished {
			return w.st, err
		}
	}
}

// walker — состояние обхода одного счёта: курсор to (конец следующего окна, идём назад), край «свежего», счётчик пустых окон.
type walker struct {
	s        *Server
	accID    string
	cfg      walkCfg
	st       walkStats
	to       int64 // конец следующего окна
	tipEdge  int64 // свежее этого края не считаем снятым
	emptyRun int   // пустых окон подряд
	opened   int64 // реальная дата открытия счёта из выписки (0 — неизвестна)
}

func (s *Server) newWalker(accID string, cfg walkCfg) *walker {
	now := cfg.now()
	to := now.Unix()
	// Закрыт (вручную) — после даты закрытия операций нет, хвост у банка не спрашиваем.
	if c := s.storage.ManualClosedAt(accID); c > 0 && c+86400 < to {
		to = c + 86400
	}
	tipEdge := now.Add(-walkTipMargin).Unix()
	if to < tipEdge {
		tipEdge = to // закрытый счёт: край не «свежий», его можно считать снятым целиком
	}
	return &walker{s: s, accID: accID, cfg: cfg, to: to, tipEdge: tipEdge, opened: s.storage.OpenedAt(accID)}
}

// complete — история счёта снята целиком до границы upTo.
func (w *walker) complete(upTo int64) {
	_ = w.s.storage.MarkHistoryComplete(w.accID, upTo)
	w.s.finish(w.accID, &w.st)
}

// step — один шаг обхода: прыжок через снятое, либо запрос окна. finished — обход счёта закончен (успешно или с ошибкой err).
func (w *walker) step() (finished bool, err error) {
	if w.cfg.stopped() {
		return true, errStopped
	}
	spans, err := w.s.storage.Coverage(w.accID)
	if err != nil {
		return true, err
	}

	// 1. Это место уже снято → прыжок без запроса.
	if sp, ok := db.SpanAt(spans, w.to); ok {
		return w.skip(sp)
	}
	// 2. Дата открытия известна из выписки → ниже неё искать нечего, ни «пустых окон», ни годов-констант.
	if w.opened > 0 && w.to < w.opened {
		w.complete(w.to)
		return true, nil
	}
	// 3. Окно, запрос, запись результата.
	from := w.windowStart(spans)
	items, done, err := w.request(from)
	if err != nil || done {
		return true, err
	}
	return w.record(items, from)
}

// skip перепрыгивает снятый отрезок sp без запроса к банку.
func (w *walker) skip(sp db.Span) (bool, error) {
	if sp.From <= db.HistoryStart {
		w.s.finish(w.accID, &w.st)
		return true, nil
	}
	// Пустой снятый отрезок идёт в счёт «пустого хвоста»: так пустой счёт (ФОП без операций)
	// не приходится заново вычерпывать год окон после каждого прерывания.
	has, err := w.s.storage.HasTransactions(w.accID, sp.From, sp.To)
	if err != nil {
		return true, err
	}
	if has {
		w.emptyRun = 0
	} else {
		w.emptyRun += int((sp.To - sp.From + 1 + int64(walkWindow/time.Second) - 1) / int64(walkWindow/time.Second))
	}
	w.to = sp.From - 1
	if w.emptyRun >= walkMaxEmpty {
		w.complete(w.to)
		return true, nil
	}
	return false, nil
}

// windowStart — начало окна: 30 дней назад, но не залезаем в уже снятое, не просим то, что до открытия, и не уходим ниже 1.
func (w *walker) windowStart(spans []db.Span) int64 {
	from := time.Unix(w.to, 0).AddDate(0, 0, -30).Unix()
	if prev, ok := db.SpanEndingBefore(spans, w.to); ok && prev.To+1 > from {
		from = prev.To + 1
	}
	if w.opened > 0 && from < w.opened {
		from = w.opened
	}
	if from < 1 {
		from = 1
	}
	return from
}

// request запрашивает окно [from, to] у банка. done — банк сказал «раньше ничего нет»: счёт снят целиком.
func (w *walker) request(from int64) (items []mono.StatementItemDTO, done bool, err error) {
	if w.cfg.onWindow != nil {
		w.cfg.onWindow(from, w.to)
	}
	w.cfg.pace()
	if w.cfg.stopped() {
		return nil, false, errStopped
	}
	items, err = w.s.fetchWithRetry(w.cfg, from, w.to)
	w.st.Requests++
	if w.cfg.stopped() { // кеш сбросили, пока банк отвечал
		return nil, false, errStopped
	}
	switch {
	case err == nil:
		return items, false, nil
	case w.cfg.classify(err) == errHistoryEnd:
		log.Printf("[Sync] %s: банк не отдаёт историю раньше — счёт %s снят целиком", w.cfg.label, w.accID)
		w.complete(w.to)
		return nil, true, nil
	default:
		return nil, false, err
	}
}

// record сохраняет полученное, отмечает в покрытии, что окно закрыто, и двигает курсор назад.
func (w *walker) record(items []mono.StatementItemDTO, from int64) (bool, error) {
	covFrom, nextTo := from, from-1
	if len(items) > 0 {
		if err := w.s.storage.SaveTransactions(w.accID, items); err != nil {
			return true, fmt.Errorf("save: %w", err)
		}
		// Банк мог упереться в лимит ответа (у Mono — 500). Тогда закрыто только то,
		// что новее самой старой полученной операции; её секунду берём ещё раз.
		if w.cfg.pageLimit == 0 || len(items) >= w.cfg.pageLimit {
			if oldest := oldestTime(items); oldest > from {
				covFrom, nextTo = oldest+1, oldest
			}
		}
		w.emptyRun = 0
	} else {
		w.emptyRun++
	}

	covTo := w.to
	if covTo > w.tipEdge {
		covTo = w.tipEdge
	}
	if covFrom <= covTo {
		if err := w.s.storage.AddCoverage(w.accID, covFrom, covTo); err != nil {
			return true, fmt.Errorf("coverage: %w", err)
		}
	}

	if w.emptyRun >= walkMaxEmpty {
		w.complete(nextTo)
		return true, nil
	}
	w.to = nextTo
	w.cfg.status(fmt.Sprintf("%s · СНЯТО %d", w.cfg.label, len(items)))
	return false, nil
}

func (s *Server) fetchWithRetry(cfg walkCfg, from, to int64) ([]mono.StatementItemDTO, error) {
	rate, net := 0, 0
	for {
		if cfg.stopped() {
			return nil, errStopped
		}
		items, err := cfg.fetch(time.Unix(from, 0), time.Unix(to, 0))
		if err == nil {
			return items, nil
		}
		switch kind := cfg.classify(err); {
		case kind == errRetry && rate < walkMaxRetries:
			rate++
			cfg.status(fmt.Sprintf("%s · 429 ЛИМИТ · ОХЛАЖДЕНИЕ %dС", cfg.label, int(cfg.retryWait/time.Second)))
			cfg.sleep(cfg.retryWait)
		case kind == errNetwork:
			// Сеть пропала: окно не теряем — ждём и повторяем то же самое. Пропадёт надолго — сдаёмся.
			net++
			if net > walkMaxNetRetries {
				log.Printf("[Sync] %s: нет связи с банком %d попыток подряд: %v", cfg.label, net-1, err)
				return nil, errOffline
			}
			cfg.status(fmt.Sprintf("%s · НЕТ СВЯЗИ · ПОВТОР ЧЕРЕЗ %dС (%d/%d)", cfg.label, int(walkNetWait/time.Second), net, walkMaxNetRetries))
			cfg.sleep(walkNetWait)
		default:
			return items, err
		}
	}
}

// newPacer — пауза ставится только между реальными запросами одного ключа
// (лимит банка действует на ключ, а не на счёт), прыжки по кешу ничего не стоят.
func newPacer(pause time.Duration, sleep func(time.Duration)) func() {
	called := false
	return func() {
		if called {
			sleep(pause)
		}
		called = true
	}
}

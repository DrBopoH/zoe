package api

import (
	"fmt"
	"log"
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
)

// walkCfg — всё, что отличает Mono от Raif (и что подменяется в тестах).
type walkCfg struct {
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

const (
	walkWindow     = 30 * 24 * time.Hour
	walkTipMargin  = 6 * time.Hour // свежий край не считаем снятым: банк мог провести операцию с опозданием
	walkMaxEmpty   = 12            // столько пустых окон подряд (≈ год) — считаем, что история закончилась
	walkMaxRetries = 20
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
	var st walkStats
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
	emptyRun := 0
	opened := s.storage.OpenedAt(accID)

	for {
		spans, err := s.storage.Coverage(accID)
		if err != nil {
			return st, err
		}

		// 1. Это место уже снято → прыжок без запроса.
		if sp, ok := db.SpanAt(spans, to); ok {
			if sp.From <= db.HistoryStart {
				s.finish(accID, &st)
				return st, nil
			}
			// Пустой снятый отрезок идёт в счёт «пустого хвоста»: так пустой счёт (ФОП без операций)
			// не приходится заново вычерпывать год окон после каждого прерывания.
			has, err := s.storage.HasTransactions(accID, sp.From, sp.To)
			if err != nil {
				return st, err
			}
			if has {
				emptyRun = 0
			} else {
				emptyRun += int((sp.To - sp.From + 1 + int64(walkWindow/time.Second) - 1) / int64(walkWindow/time.Second))
			}
			to = sp.From - 1
			if emptyRun >= walkMaxEmpty {
				_ = s.storage.MarkHistoryComplete(accID, to)
				s.finish(accID, &st)
				return st, nil
			}
			continue
		}

		// 2. Дата открытия известна из выписки → ниже неё искать нечего, ни «пустых окон», ни годов-констант.
		if opened > 0 && to < opened {
			_ = s.storage.MarkHistoryComplete(accID, to)
			s.finish(accID, &st)
			return st, nil
		}

		// 3. Окно: 30 дней назад, но не залезаем в уже снятое и не уходим ниже 2017.
		from := time.Unix(to, 0).AddDate(0, 0, -30).Unix()
		if prev, ok := db.SpanEndingBefore(spans, to); ok && prev.To+1 > from {
			from = prev.To + 1
		}
		if opened > 0 && from < opened {
			from = opened // не просим у банка то, что до открытия
		}
		if from < 1 {
			from = 1
		}

		if cfg.onWindow != nil {
			cfg.onWindow(from, to)
		}
		cfg.pace()
		items, err := s.fetchWithRetry(cfg, from, to)
		st.Requests++
		switch {
		case err == nil:
		case cfg.classify(err) == errHistoryEnd:
			log.Printf("[Sync] %s: банк не отдаёт историю раньше — счёт %s снят целиком", cfg.label, accID)
			_ = s.storage.MarkHistoryComplete(accID, to)
			s.finish(accID, &st)
			return st, nil
		default:
			return st, err
		}

		// 4. Что именно этот запрос закрыл.
		covFrom, nextTo := from, from-1
		if len(items) > 0 {
			if err := s.storage.SaveTransactions(accID, items); err != nil {
				return st, fmt.Errorf("save: %w", err)
			}
			// Банк мог упереться в лимит ответа (у Mono — 500). Тогда закрыто только то,
			// что новее самой старой полученной операции; её секунду берём ещё раз.
			if cfg.pageLimit == 0 || len(items) >= cfg.pageLimit {
				oldest := oldestTime(items)
				if oldest > from {
					covFrom, nextTo = oldest+1, oldest
				}
			}
			emptyRun = 0
		} else {
			emptyRun++
		}

		covTo := to
		if covTo > tipEdge {
			covTo = tipEdge
		}
		if covFrom <= covTo {
			if err := s.storage.AddCoverage(accID, covFrom, covTo); err != nil {
				return st, fmt.Errorf("coverage: %w", err)
			}
		}

		if emptyRun >= walkMaxEmpty {
			_ = s.storage.MarkHistoryComplete(accID, nextTo)
			s.finish(accID, &st)
			return st, nil
		}
		to = nextTo

		cfg.status(fmt.Sprintf("%s · СНЯТО %d", cfg.label, len(items)))
	}
}

func (s *Server) fetchWithRetry(cfg walkCfg, from, to int64) ([]mono.StatementItemDTO, error) {
	for attempt := 0; ; attempt++ {
		items, err := cfg.fetch(time.Unix(from, 0), time.Unix(to, 0))
		if err != nil && cfg.classify(err) == errRetry && attempt < walkMaxRetries {
			cfg.status(fmt.Sprintf("%s · 429 ЛИМИТ · ОХЛАЖДЕНИЕ %dС", cfg.label, int(cfg.retryWait/time.Second)))
			cfg.sleep(cfg.retryWait)
			continue
		}
		return items, err
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

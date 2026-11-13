package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"economic/pkg/mono"
	"economic/pkg/raif"
)

// oldestTime — самое раннее время в выписке (не полагаемся на порядок, в котором банк отдал операции).
func oldestTime(items []mono.StatementItemDTO) int64 {
	oldest := items[0].Time
	for _, it := range items[1:] {
		if it.Time < oldest {
			oldest = it.Time
		}
	}
	return oldest
}

func (s *Server) updateStatus(msg string) {
	s.syncMutex.Lock()
	s.syncStatus = msg
	s.syncMutex.Unlock()
	log.Printf("[Sync Status] %s", msg)
}

// sleepCtx — пауза, которую прерывает отмена обхода.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

func (s *Server) runFullCrawler(ctx context.Context, done chan struct{}, save bool) {
	s.prog.begin()
	defer close(done)
	defer func() {
		if ctx.Err() != nil { // сброс кеша уже снял состояние
			return
		}
		s.prog.finish()
		s.syncMutex.Lock()
		s.isSyncing = false
		s.syncStatus = "СНЯТО"
		s.syncMutex.Unlock()
	}()

	hasBinance := s.bn.client != nil && s.storage.HasBinance()
	if len(s.monoClients) == 0 && len(s.raifClients) == 0 && !hasBinance {
		s.updateStatus("ОТСУТСТВУЮТ КЛЮЧИ API")
		return
	}

	// Все источники опрашиваются параллельно: у каждого ключа свой лимит запросов.
	var wg sync.WaitGroup
	run := func(f func()) {
		wg.Add(1)
		go func() { defer wg.Done(); f() }()
	}
	for i, c := range s.monoClients {
		run(func() { s.crawlBank(ctx, s.monoSpec(c, i+1, len(s.monoClients))) })
	}
	for i, c := range s.raifClients {
		run(func() { s.crawlBank(ctx, s.raifSpec(c, i+1, len(s.raifClients))) })
	}
	if hasBinance {
		run(func() { s.crawlBinance(ctx) })
	}
	wg.Wait()
	if save {
		s.saveToSources(ctx)
	}
}

// saveToSources — «Разведать» с галочкой: после обхода снятое с API дописывается в файлы-источники, чтобы кеш всегда
// восстанавливался с диска, а снятые отрезки заново не качались. Отменённый обход (сброс кеша) ничего не пишет.
func (s *Server) saveToSources(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	s.updateStatus("ЗАПИСЬ В ИСТОЧНИКИ")
	res, err := s.saveAPISources(nil)
	var errs []progError
	if err != nil {
		errs = append(errs, progError{"Источники", err.Error()})
	}
	saved := 0
	for _, r := range res {
		if r.Saved {
			saved++
		}
		if r.Error != "" {
			errs = append(errs, progError{"Источники · " + r.Name, r.Error})
		}
	}
	s.prog.saveDone(saved, errs)
}

// bankSpec — всё, чем один банковский ключ отличается от другого; сам обход (crawlBank) общий.
type bankSpec struct {
	kind       string // mono | raif — тип клиента в панели прогресса и префикс статусов
	num, total int
	tag        string        // суффикс статусов («RAIF»)
	pause      time.Duration // пауза между запросами к ключу
	retryWait  time.Duration
	pageLimit  int // лимит ответа выписки (0 — неизвестен)
	info       func() (name string, accounts []mono.AccountDTO, err error)
	statement  func(accID string, from, to time.Time) ([]mono.StatementItemDTO, error)
	classify   func(error) errKind
}

func (sp bankSpec) fallbackName() string {
	return fmt.Sprintf("%s %d", strings.ToUpper(sp.kind[:1])+sp.kind[1:], sp.num)
}

func (sp bankSpec) suffix() string {
	if sp.tag == "" {
		return ""
	}
	return " " + sp.tag
}

func (s *Server) monoSpec(client *mono.Client, num, total int) bankSpec {
	return bankSpec{
		kind: "mono", num: num, total: total,
		pause: 61 * time.Second, retryWait: 65 * time.Second, pageLimit: 500, // лимит Monobank на ответ выписки
		info: func() (string, []mono.AccountDTO, error) {
			var info *mono.UserInfoDTO
			var err error
			for attempt := 0; attempt < 3; attempt++ {
				info, err = client.GetClientInfo()
				if _, isRate := err.(*mono.RateLimitError); isRate {
					s.updateStatus(fmt.Sprintf("MONO %d/%d: 429 ЛИМИТ · ОХЛАЖДЕНИЕ 65С", num, total))
					time.Sleep(65 * time.Second)
					continue
				}
				break
			}
			if err != nil {
				return "", nil, err
			}
			return info.Name, info.Accounts, nil
		},
		statement: client.GetStatement,
		classify: func(err error) errKind {
			switch err.(type) {
			case *mono.RateLimitError:
				return errRetry
			case *mono.BadRequestError:
				return errHistoryEnd
			}
			return networkOrFatal(err)
		},
	}
}

func (s *Server) raifSpec(client *raif.Client, num, total int) bankSpec {
	return bankSpec{
		kind: "raif", num: num, total: total, tag: "RAIF",
		pause: 30 * time.Second, retryWait: 30 * time.Second, pageLimit: 0, // лимит ответа Raif неизвестен — считаем осторожно
		info:      client.GetUserInfo,
		statement: client.GetStatement,
		classify:  networkOrFatal,
	}
}

func networkOrFatal(err error) errKind {
	if isNetworkErr(err) {
		return errNetwork
	}
	return errFatal
}

// crawlBank — «Разведать» для одного ключа любого банка: счета → закрытые → выписка каждого счёта окнами.
func (s *Server) crawlBank(ctx context.Context, sp bankSpec) {
	kindUp := strings.ToUpper(sp.kind)
	ownerName, accounts, err := sp.info()
	if err != nil {
		log.Printf("[Sync Error] %s client %d info: %v", kindUp, sp.num, err)
		s.updateStatus(fmt.Sprintf("%s %d/%d: ОТКАЗ API", kindUp, sp.num, sp.total))
		s.prog.clientError(s.prog.client(sp.fallbackName(), sp.kind, sp.pause), bankErrText(err))
		return
	}
	if ownerName == "" {
		ownerName = sp.fallbackName()
	}
	if ctx.Err() != nil { // кеш сбросили, пока банк отвечал: его ответ в чистую базу не пишем
		return
	}
	if err := s.storage.SaveAccounts(accounts, ownerName); err != nil {
		log.Printf("[Sync Error] [%s] Save accounts: %v", ownerName, err)
	}
	if len(accounts) == 0 {
		s.updateStatus(fmt.Sprintf("[%s] СЧЕТА ПУСТЫ%s", ownerName, sp.suffix()))
		return
	}

	cp := s.prog.client(ownerName, sp.kind, sp.pause)
	progs := progAccounts(accounts)
	s.prog.setAccounts(cp, progs)
	ids := make([]string, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.ID)
	}
	if err := s.storage.MarkClosedAbsent(ownerName, ids); err != nil {
		log.Printf("[Sync Error] [%s] closed accounts: %v", ownerName, err)
	}

	var cur *accProg
	coolSleep := func(d time.Duration) { s.prog.cool(cur, d); sleepCtx(ctx, d); s.prog.fetching(cur) }
	pace := newPacer(sp.pause, coolSleep)
	for i, acc := range accounts {
		if ctx.Err() != nil {
			return
		}
		cur = progs[i]
		label := fmt.Sprintf("[%s] СЧЕТ %d/%d (%s)%s", ownerName, i+1, len(accounts), acc.Type, sp.suffix())
		s.updateStatus(label)

		accID := acc.ID
		st, err := s.walkAccount(accID, walkCfg{
			ctx:       ctx,
			label:     label,
			pageLimit: sp.pageLimit,
			pause:     sp.pause,
			retryWait: sp.retryWait,
			classify:  sp.classify,
			fetch:     func(from, to time.Time) ([]mono.StatementItemDTO, error) { return sp.statement(accID, from, to) },
			sleep:     coolSleep,
			pace:      pace,
			onWindow:  func(f, t int64) { s.prog.window(cur, f, t) },
			now:       time.Now,
			status:    s.updateStatus,
		})
		if errors.Is(err, errStopped) {
			return
		}
		s.prog.accDone(cur, st, err)
		if errors.Is(err, errOffline) {
			s.abortOffline(progs, i+1)
			break
		}
		if err != nil {
			log.Printf("[Sync Error] [%s] счёт %s: %v", ownerName, acc.ID, err)
			s.updateStatus(fmt.Sprintf("[%s] СБОЙ ВЫПИСКИ%s", acc.Type, sp.suffix()))
			continue
		}
		log.Printf("[Sync] [%s] счёт %s: запросов %d, история целиком: %v", ownerName, acc.ID, st.Requests, st.Complete)
	}
}

// bankErrText — текст ошибки для панели: сеть и отказ банка различаем.
func bankErrText(err error) string {
	if isNetworkErr(err) {
		return errOffline.Error()
	}
	return "банк отказал: " + err.Error()
}

// abortOffline — сеть пропала: остальные счета ключа помечаем ошибкой и не опрашиваем.
func (s *Server) abortOffline(progs []*accProg, from int) {
	for j := from; j < len(progs); j++ {
		s.prog.accDone(progs[j], walkStats{}, errOffline)
	}
}

package api

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"economic/pkg/db"
	"economic/pkg/mono"
)

// ───────── фейковый банк ─────────

type fakeBank struct {
	txs     []mono.StatementItemDTO // любые, порядок не важен
	opened  int64                   // до этого момента банк отвечает BadRequest
	limit   int
	calls   [][2]int64
	failAt  int // 0 — не падать; N — N-й запрос вернёт fatal
	counter int
}

var errBoom = errors.New("boom")

func (b *fakeBank) fetch(from, to time.Time) ([]mono.StatementItemDTO, error) {
	b.counter++
	b.calls = append(b.calls, [2]int64{from.Unix(), to.Unix()})
	if b.failAt > 0 && b.counter == b.failAt {
		return nil, errBoom
	}
	if to.Unix() < b.opened {
		return nil, &mono.BadRequestError{}
	}
	var out []mono.StatementItemDTO
	for _, t := range b.txs {
		if t.Time >= from.Unix() && t.Time <= to.Unix() {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time }) // новые первыми, как у Mono
	if b.limit > 0 && len(out) > b.limit {
		out = out[:b.limit]
	}
	return out, nil
}

func (b *fakeBank) add(ts int64) {
	b.txs = append(b.txs, mono.StatementItemDTO{ID: fmt.Sprintf("t%d", ts), Time: ts, Amount: -100})
}

// ───────── окружение ─────────

type env struct {
	t     *testing.T
	s     *Server
	st    *db.Storage
	bank  *fakeBank
	now   time.Time
	sleep int
}

func newEnv(t *testing.T) *env {
	d := t.TempDir()
	st, err := db.NewStorage(filepath.Join(d, "cache.db"), filepath.Join(d, "core.db"))
	if err != nil {
		t.Fatal(err)
	}
	return &env{t: t, s: NewServer(st, nil, nil), st: st,
		bank: &fakeBank{limit: 500}, now: time.Date(2026, 12, 30, 12, 0, 0, 0, time.UTC)}
}

func (e *env) run() walkStats {
	pace := newPacer(61*time.Second, func(time.Duration) { e.sleep++ })
	st, err := e.s.walkAccount("acc", walkCfg{
		label: "test", pageLimit: 500, pause: 61 * time.Second, retryWait: time.Second,
		classify: func(err error) errKind {
			if _, ok := err.(*mono.BadRequestError); ok {
				return errHistoryEnd
			}
			return errFatal
		},
		fetch: e.bank.fetch, sleep: func(time.Duration) {}, pace: pace,
		now: func() time.Time { return e.now }, status: func(string) {},
	})
	if err != nil && !errors.Is(err, errBoom) {
		e.t.Fatalf("walk: %v", err)
	}
	return st
}

func (e *env) cached() map[string]bool {
	txs, err := e.st.GetTransactions(1_000_000)
	if err != nil {
		e.t.Fatal(err)
	}
	m := map[string]bool{}
	for _, t := range txs {
		m[t.ID] = true
	}
	return m
}

func (e *env) assertAllCached() {
	e.t.Helper()
	have := e.cached()
	for _, tx := range e.bank.txs {
		if !have[tx.ID] {
			e.t.Fatalf("tx %s (%s) missing in cache", tx.ID, time.Unix(tx.Time, 0).UTC().Format("2006-01-02"))
		}
	}
	if len(have) != len(e.bank.txs) {
		e.t.Fatalf("cache has %d, bank %d", len(have), len(e.bank.txs))
	}
}

// счёт открыт 2024-03-15, по операции в сутки до сегодняшнего дня
func (e *env) populate() {
	start := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	e.bank.opened = start.Unix() - 86400*20
	for t := start; t.Before(e.now); t = t.Add(24 * time.Hour) {
		e.bank.add(t.Unix())
	}
}

func (e *env) advance(d time.Duration) {
	e.now = e.now.Add(d)
	for t := e.now.Add(-d).Add(24 * time.Hour); t.Before(e.now); t = t.Add(24 * time.Hour) {
		e.bank.add(t.Unix())
	}
}

// ───────── сценарии ─────────

// Пустой кеш → снимаем всё до открытия счёта, помечаем «история целиком».
func TestFullCrawl(t *testing.T) {
	e := newEnv(t)
	e.populate()
	st := e.run()
	if !st.Complete {
		t.Fatal("history must be complete")
	}
	e.assertAllCached()
	t.Logf("full crawl: %d requests", st.Requests)
}

// Сценарий 1: кеш свежий → один запрос к хвосту и выход.
func TestUpToDateOneRequest(t *testing.T) {
	e := newEnv(t)
	e.populate()
	e.run()

	e.bank.calls = nil
	e.advance(3 * time.Hour)
	st := e.run()
	if st.Requests != 1 || !st.Complete {
		t.Fatalf("expected 1 request + complete, got %+v calls=%v", st, e.bank.calls)
	}
	e.assertAllCached()
}

// Сценарий 2: кеш отстал на ~2 месяца, история от открытия карты есть →
// только дыра (3 окна), стык с кешем склеивается, дальше прыжок без запросов.
func TestStaleGapThenStop(t *testing.T) {
	e := newEnv(t)
	e.populate()
	e.run()

	e.bank.calls = nil
	e.advance(68 * 24 * time.Hour)
	st := e.run()
	if !st.Complete {
		t.Fatal("must finish complete")
	}
	if st.Requests < 2 || st.Requests > 4 {
		t.Fatalf("expected ~3 requests for 68 days gap, got %d: %v", st.Requests, e.bank.calls)
	}
	e.assertAllCached()
}

// Сценарий 3: выгрузка оборвалась посреди истории, потом кеш отстал →
// свежее окно + прыжок через уже снятое + продолжение с последней дыры. Снятое второй раз не запрашивается.
func TestInterruptedThenStale(t *testing.T) {
	e := newEnv(t)
	e.populate()
	e.bank.failAt = 11 // 10 окон снято, на 11-м обрыв
	st := e.run()
	if st.Complete {
		t.Fatal("interrupted run must not be complete")
	}
	before, _ := e.st.Coverage("acc")
	if len(before) != 1 {
		t.Fatalf("expected one covered span, got %v", before)
	}

	e.bank.failAt, e.bank.counter, e.bank.calls = 0, 0, nil
	e.advance(60 * 24 * time.Hour)
	st = e.run()
	if !st.Complete {
		t.Fatal("second run must complete")
	}
	e.assertAllCached()

	// ни один запрос не должен заходить в уже снятое глубже, чем на край (секунда стыка)
	for _, c := range e.bank.calls {
		for _, sp := range before {
			lo, hi := max64(c[0], sp.From), min64(c[1], sp.To)
			if hi-lo > 1 {
				t.Fatalf("call %v re-fetches covered span %v", c, sp)
			}
		}
	}
	t.Logf("resume: %d requests", st.Requests)
}

// Операция, проведённая банком с опозданием (время в прошлом относительно прошлой синхронизации).
func TestLateArrivingTx(t *testing.T) {
	e := newEnv(t)
	e.populate()
	e.run()

	late := e.now.Add(-2*time.Hour - 7*time.Minute).Unix() // «случилась» за 2 часа до прошлой синхронизации, но банк отдал её позже
	e.bank.add(late)
	e.advance(5 * time.Hour)
	e.run()
	e.assertAllCached()
}

// Лимит ответа 500: плотное окно разбирается несколькими запросами, ничего не теряется.
func TestDenseWindow(t *testing.T) {
	e := newEnv(t)
	e.bank.opened = e.now.Add(-100 * 24 * time.Hour).Unix()
	for i := 0; i < 1300; i++ {
		e.bank.add(e.now.Add(-time.Duration(i+1) * 30 * time.Minute).Unix())
	}
	st := e.run()
	if !st.Complete {
		t.Fatal("not complete")
	}
	e.assertAllCached()
}

// Снос кеша стирает и память о покрытии → всё снимается заново.
func TestWipeForgetsCoverage(t *testing.T) {
	e := newEnv(t)
	e.populate()
	e.run()
	if err := e.st.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if cov, _ := e.st.Coverage("acc"); len(cov) != 0 {
		t.Fatalf("coverage survived wipe: %v", cov)
	}
	e.bank.calls = nil
	st := e.run()
	if st.Requests < 10 {
		t.Fatalf("after wipe must re-crawl, got %d requests", st.Requests)
	}
	e.assertAllCached()
}

// Пустой счёт (ФОП без операций): после прерывания не вычерпываем год окон заново.
func TestEmptyAccountResumes(t *testing.T) {
	e := newEnv(t)
	e.bank.opened = 0
	e.bank.failAt = 9 // 8 пустых окон снято, на 9-м обрыв
	if st := e.run(); st.Complete {
		t.Fatal("interrupted must not be complete")
	}
	e.bank.failAt, e.bank.counter, e.bank.calls = 0, 0, nil
	st := e.run()
	if !st.Complete || st.Requests > 6 {
		t.Fatalf("resume: complete=%v requests=%d", st.Complete, st.Requests)
	}
	// третий запуск: всё уже «снято целиком» — один запрос к хвосту
	e.bank.calls = nil
	if st = e.run(); st.Requests != 1 || !st.Complete {
		t.Fatalf("third run: %+v", st)
	}
}

// Паузы ставятся только между реальными запросами, а не на прыжках по кешу.
func TestPaceOnlyBetweenRequests(t *testing.T) {
	e := newEnv(t)
	e.populate()
	e.run()
	e.sleep = 0
	e.advance(time.Hour)
	st := e.run()
	if st.Requests != 1 || e.sleep != 0 {
		t.Fatalf("requests=%d sleeps=%d", st.Requests, e.sleep)
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// Дата открытия берётся из выписки и пишется в кеш; закрытие — счёт пропал из ответа банка.
func TestAccountLife(t *testing.T) {
	e := newEnv(t)
	e.populate()
	if err := e.st.SaveAccounts([]mono.AccountDTO{{ID: "acc", Type: "black"}, {ID: "old", Type: "white"}}, "Owner"); err != nil {
		t.Fatal(err)
	}
	e.run()
	var opened int64
	accs, _ := e.st.GetAccounts()
	for _, a := range accs {
		if a.ID == "acc" {
			opened = a.OpenedAt
		}
	}
	want := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC).Unix()
	if opened != want {
		t.Fatalf("opened_at=%d want %d", opened, want)
	}
	// банк больше не отдаёт «old»
	if err := e.st.MarkClosedAbsent("Owner", []string{"acc"}); err != nil {
		t.Fatal(err)
	}
	accs, _ = e.st.GetAccounts()
	for _, a := range accs {
		if (a.ID == "old") != a.IsClosed {
			t.Fatalf("%s closed=%v", a.ID, a.IsClosed)
		}
	}
	// вернулся → снова жив
	_ = e.st.SaveAccounts([]mono.AccountDTO{{ID: "acc", Type: "black"}, {ID: "old", Type: "white"}}, "Owner")
	accs, _ = e.st.GetAccounts()
	for _, a := range accs {
		if a.IsClosed {
			t.Fatalf("%s must be alive", a.ID)
		}
	}
}

func TestManualLife(t *testing.T) {
	e := newEnv(t)
	_ = e.st.SaveAccounts([]mono.AccountDTO{{ID: "acc", Type: "black"}}, "Owner")
	opened := time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC).Unix()
	closed := time.Date(2023, 2, 3, 0, 0, 0, 0, time.UTC).Unix()
	if err := e.st.SetAccountLife("acc", opened, closed); err != nil {
		t.Fatal(err)
	}
	get := func() db.Account { a, _ := e.st.GetAccounts(); return a[0] }
	if a := get(); a.OpenedAt != opened || a.ClosedAt != closed || !a.IsClosed || !a.ManualOpened {
		t.Fatalf("%+v", a)
	}
	// синк (банк отдаёт счёт) ручное закрытие не сбрасывает
	_ = e.st.SaveAccounts([]mono.AccountDTO{{ID: "acc", Type: "black"}}, "Owner")
	if a := get(); !a.IsClosed {
		t.Fatal("manual close lost after sync")
	}
	// очистка → снова автомат
	_ = e.st.SetAccountLife("acc", 0, 0)
	if a := get(); a.IsClosed || a.ManualOpened {
		t.Fatalf("%+v", a)
	}
}

// Ручные даты реально ограничивают синк: ниже открытия и выше закрытия банк не спрашиваем.
func TestManualLifeBoundsSync(t *testing.T) {
	e := newEnv(t)
	e.populate() // операции с 2024-03-15 до «сейчас»
	_ = e.st.SaveAccounts([]mono.AccountDTO{{ID: "acc", Type: "black"}}, "Owner")
	opened := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC).Unix()
	closed := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC).Unix()
	_ = e.st.SetAccountLife("acc", opened, closed)
	st := e.run()
	if !st.Complete {
		t.Fatal("must complete")
	}
	for _, c := range e.bank.calls {
		if c[1] > closed+86400 {
			t.Fatalf("asked above closing date: %v", c)
		}
		if c[0] < opened {
			t.Fatalf("asked below opening date: %v", c)
		}
	}
	if st.Requests > 5 {
		t.Fatalf("too many requests: %d", st.Requests)
	}
}

// Снимок прогресса: ось, клетки таймлайна (с операциями / пустые / не снято), проценты, окно и ETA.
func TestProgressSnapshot(t *testing.T) {
	e := newEnv(t)
	e.populate()
	acc := &accProg{ID: "acc", Label: "UA00****0000 · black", State: "queued"}
	cp := e.s.prog.client("Owner", "mono", 61*time.Second)
	e.s.prog.setAccounts(cp, []*accProg{acc})
	e.s.prog.begin()
	cp = e.s.prog.client("Owner", "mono", 61*time.Second)
	e.s.prog.setAccounts(cp, []*accProg{acc})

	v := e.s.progressSnapshot()
	if !v.Running || len(v.Clients) != 1 || v.Clients[0].Accounts[0].Percent != 0 || v.ETA == 0 {
		t.Fatalf("fresh: %+v", v)
	}

	e.run()
	e.s.prog.accDone(acc, walkStats{Requests: 3, Complete: true}, nil)
	v = e.s.progressSnapshot()
	av := v.Clients[0].Accounts[0]
	if av.Percent != 100 || len(av.Cells) != timelineCells || v.Clients[0].Percent != 100 {
		t.Fatalf("done: %d%% cells=%d", av.Percent, len(av.Cells))
	}
	if av.Cells[timelineCells/2] != '1' {
		t.Fatalf("middle must be covered with txs: %s", av.Cells)
	}
	e.s.prog.finish()
	if v = e.s.progressSnapshot(); v.Running || v.Last == nil || v.Last.Requests != 3 {
		t.Fatalf("finish: %+v", v.Last)
	}
}

// Пустой счёт: снято, но операций не было → клетки «2» (косая штриховка на фронте).
func TestProgressEmptyCells(t *testing.T) {
	e := newEnv(t)
	e.bank.opened = 0
	acc := &accProg{ID: "acc", State: "queued"}
	e.s.prog.begin()
	cp := e.s.prog.client("Owner", "mono", 61*time.Second)
	e.s.prog.setAccounts(cp, []*accProg{acc})
	e.run()
	e.s.prog.accDone(acc, walkStats{Complete: true}, nil)
	av := e.s.progressSnapshot().Clients[0].Accounts[0]
	if !strings.Contains(av.Cells, "2") || strings.Contains(av.Cells, "1") {
		t.Fatalf("expected empty-covered cells: %s", av.Cells)
	}
}

// ETA не завышается: пустой счёт после обрыва знает, что 8 пустых окон уже сняты.
func TestProgressETARemaining(t *testing.T) {
	e := newEnv(t)
	e.bank.opened = 0
	e.bank.failAt = 9
	acc := &accProg{ID: "acc", State: "queued"}
	e.s.prog.begin()
	cp := e.s.prog.client("Owner", "mono", 61*time.Second)
	e.s.prog.setAccounts(cp, []*accProg{acc})
	e.run()
	av := e.s.progressSnapshot().Clients[0].Accounts[0]
	if av.remaining < 3 || av.remaining > 6 {
		t.Fatalf("remaining=%d, want about 4-5 (12 - 8 empty windows already covered + tip)", av.remaining)
	}
}

// Обрыв сети: окно не теряется — обход ждёт и повторяет то же окно, пока связь не вернётся.
func TestNetworkOutageRecovers(t *testing.T) {
	e := newEnv(t)
	down := 3
	slept := 0
	netErr := &net.OpError{Op: "dial", Err: errors.New("no such host")}
	inner := e.bank.fetch
	_, err := e.s.walkAccount("acc", walkCfg{
		label: "test", pageLimit: 500, pause: 61 * time.Second, retryWait: time.Second,
		classify: func(err error) errKind {
			if isNetworkErr(err) {
				return errNetwork
			}
			if _, ok := err.(*mono.BadRequestError); ok {
				return errHistoryEnd
			}
			return errFatal
		},
		fetch: func(from, to time.Time) ([]mono.StatementItemDTO, error) {
			if down > 0 {
				down--
				return nil, netErr
			}
			return inner(from, to)
		},
		sleep: func(time.Duration) { slept++ }, pace: func() {},
		now: func() time.Time { return e.now }, status: func(string) {},
	})
	if err != nil || slept != 3 {
		t.Fatalf("err=%v slept=%d", err, slept)
	}
}

// Связи нет слишком долго: ключ останавливается с errOffline, а не висит вечно.
func TestNetworkOutageGivesUp(t *testing.T) {
	e := newEnv(t)
	calls := 0
	_, err := e.s.walkAccount("acc", walkCfg{
		label: "test", pageLimit: 500, pause: 61 * time.Second, retryWait: time.Second,
		classify: func(err error) errKind { return errNetwork },
		fetch: func(from, to time.Time) ([]mono.StatementItemDTO, error) {
			calls++
			return nil, &net.OpError{Op: "dial", Err: errors.New("down")}
		},
		sleep: func(time.Duration) {}, pace: func() {},
		now: func() time.Time { return e.now }, status: func(string) {},
	})
	if !errors.Is(err, errOffline) || calls != walkMaxNetRetries+1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

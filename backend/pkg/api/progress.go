package api

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"economic/pkg/db"
	"economic/pkg/mono"
)

// Структурированный прогресс синка: клиенты (владельцы ключей) → счета → таймлайн снятого.
// Фронт получает это в /api/sync/status и рисует панель; никаких строк статуса парсить не нужно.

const timelineCells = 96

type progWin struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

type accProg struct {
	ID        string
	Label     string
	State     string // queued | fetch | cooling | done | error
	Win       *progWin
	CoolUntil int64
	Requests  int
	Error     string
	Complete  bool

	// счета без таймлайна покрытия (Binance): прогресс и подпись задаёт сам источник
	Custom   bool
	Pct      int
	Note     string
	AxisFrom int64   // начало оси (unix); 0 — горизонт поиска
	FlowAt   []int64 // моменты движений (unix) — клетки с ними зелёные, остальные косые
	Left     int     // сколько запросов осталось (монеты цен)
}

type clientProg struct {
	Name     string
	Kind     string // mono | raif
	Pause    time.Duration
	ReqSec   float64 // средняя длительность запроса, сек (клиенты без фиксированной паузы: Binance)
	Error    string
	Finished bool
	Accounts []*accProg
}

type progError struct {
	Who string `json:"who"`
	Msg string `json:"msg"`
}

type syncResult struct {
	FinishedAt int64       `json:"finished_at"`
	Requests   int         `json:"requests"`
	Errors     []progError `json:"errors"`
}

type progress struct {
	mu        sync.Mutex
	running   bool
	startedAt int64
	clients   []*clientProg
	last      *syncResult
}

func (p *progress) begin() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running, p.startedAt, p.clients = true, time.Now().Unix(), nil
}

// reset убирает панель прогресса: «Разведать» снят сбросом кеша, показывать нечего.
func (p *progress) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running, p.clients, p.last = false, nil, nil
}

func (p *progress) client(name, kind string, pause time.Duration) *clientProg {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.clients {
		if c.Name == name && c.Kind == kind {
			return c
		}
	}
	c := &clientProg{Name: name, Kind: kind, Pause: pause}
	p.clients = append(p.clients, c)
	return c
}

func (p *progress) setAccounts(c *clientProg, list []*accProg) {
	p.mu.Lock()
	c.Accounts = list
	p.mu.Unlock()
}

func (p *progress) clientError(c *clientProg, msg string) {
	p.mu.Lock()
	c.Error, c.Finished = msg, true
	p.mu.Unlock()
}

func (p *progress) with(a *accProg, f func(*accProg)) {
	p.mu.Lock()
	f(a)
	p.mu.Unlock()
}

func (p *progress) window(a *accProg, from, to int64) {
	p.with(a, func(a *accProg) { a.Win, a.State, a.CoolUntil = &progWin{from, to}, "fetch", 0 })
}

func (p *progress) cool(a *accProg, d time.Duration) {
	p.with(a, func(a *accProg) { a.State, a.CoolUntil = "cooling", time.Now().Add(d).Unix() })
}

func (p *progress) fetching(a *accProg) {
	p.with(a, func(a *accProg) { a.State, a.CoolUntil = "fetch", 0 })
}

func (p *progress) accDone(a *accProg, st walkStats, err error) {
	p.with(a, func(a *accProg) {
		a.Requests, a.Complete, a.CoolUntil = st.Requests, st.Complete, 0
		if err == nil {
			a.Win = nil // успех — окно больше не нужно; при ошибке оставляем: фронт подсветит период, на котором упали
		}
		if err != nil {
			a.State, a.Error = "error", err.Error()
		} else {
			a.State = "done"
		}
	})
}

func (p *progress) finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	res := &syncResult{FinishedAt: time.Now().Unix(), Errors: []progError{}}
	for _, c := range p.clients {
		c.Finished = true
		if c.Error != "" {
			res.Errors = append(res.Errors, progError{c.Name, c.Error})
		}
		for _, a := range c.Accounts {
			res.Requests += a.Requests
			if a.Error != "" {
				res.Errors = append(res.Errors, progError{c.Name + " · " + a.Label, a.Error})
			}
		}
	}
	p.running, p.last = false, res
}

// ───────── снимок для API ─────────

type accView struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	State     string   `json:"state"`
	Percent   int      `json:"percent"`
	AxisFrom  int64    `json:"axis_from"`
	AxisTo    int64    `json:"axis_to"`
	Cells     string   `json:"cells"` // 0 — не снято, 1 — снято, есть операции, 2 — снято, операций не было
	Win       *progWin `json:"window,omitempty"`
	CoolUntil int64    `json:"cool_until,omitempty"`
	Error     string   `json:"error,omitempty"`
	Note      string   `json:"note,omitempty"` // подпись под таймлайном (этап)
	remaining int
}

type clientView struct {
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	Percent  int       `json:"percent"`
	Error    string    `json:"error,omitempty"`
	Finished bool      `json:"finished"`
	Accounts []accView `json:"accounts"`
}

type progressView struct {
	Running   bool         `json:"running"`
	StartedAt int64        `json:"started_at"`
	ETA       int64        `json:"eta"` // unix, примерно; 0 — нечего ждать
	Clients   []clientView `json:"clients"`
	Last      *syncResult  `json:"last,omitempty"`
}

func (s *Server) progressSnapshot() progressView {
	now := time.Now().Unix()
	s.prog.mu.Lock()
	type cp struct {
		c    clientProg
		accs []accProg
		win  []*progWin
	}
	var cps []cp
	for _, c := range s.prog.clients {
		x := cp{c: *c}
		for _, a := range c.Accounts {
			cpy := *a
			if a.Win != nil {
				w := *a.Win
				cpy.Win = &w
			}
			x.accs = append(x.accs, cpy)
		}
		cps = append(cps, x)
	}
	view := progressView{Running: s.prog.running, StartedAt: s.prog.startedAt, Last: s.prog.last, Clients: []clientView{}}
	s.prog.mu.Unlock()

	var eta int64
	for _, x := range cps {
		cv := clientView{Name: x.c.Name, Kind: x.c.Kind, Error: x.c.Error, Finished: x.c.Finished, Accounts: []accView{}}
		sum, remaining, settled := 0, 0, 0
		for i := range x.accs {
			av := s.accTimeline(&x.accs[i], now)
			sum += av.Percent
			if st := x.accs[i].State; st == "done" || st == "error" {
				settled++
			}
			if x.accs[i].State != "done" && x.accs[i].State != "error" && x.c.Error == "" {
				remaining += av.remaining
			}
			cv.Accounts = append(cv.Accounts, av)
		}
		if n := len(x.accs); n > 0 {
			cv.Percent = sum / n
			if settled == n {
				cv.Finished = true
			}
		} else if x.c.Finished {
			cv.Percent = 100
		}
		if remaining > 0 {
			// пауза ставится только между запросами: перед первым ждать не нужно; ~2 с на сам запрос
			wait := int64(remaining-1)*int64(x.c.Pause/time.Second) + int64(remaining)*2
			if x.c.Kind == "binance" { // у Binance нет фиксированной паузы: считаем по измеренной скорости запросов
				sec := x.c.ReqSec
				if sec <= 0 {
					sec = 0.7
				}
				wait = int64(float64(remaining) * sec)
			}
			if t := now + wait; t > eta {
				eta = t
			}
		}
		view.Clients = append(view.Clients, cv)
	}
	if view.Running {
		view.ETA = eta
	}
	return view
}

// accTimeline строит ось счёта [открытие … сейчас] и раскрашивает её по таблице покрытия.
func (s *Server) accTimeline(a *accProg, now int64) accView {
	if a.Custom {
		return s.customAccView(a, now)
	}
	av := accView{ID: a.ID, Label: a.Label, State: a.State, Win: a.Win, CoolUntil: a.CoolUntil, Error: a.Error}
	win := int64(walkWindow / time.Second)

	spans, _ := s.storage.Coverage(a.ID)
	txs, _ := s.storage.TxTimes(a.ID)
	complete := false
	for _, sp := range spans {
		if sp.From <= db.HistoryStart {
			complete = true
		}
	}
	hi := now
	if c := s.storage.ManualClosedAt(a.ID); c > 0 && c+86400 < hi {
		hi = c + 86400
	}
	lo := hi
	if o := s.storage.OpenedAt(a.ID); o > 0 {
		lo = o
	} else {
		if len(txs) > 0 && txs[0] < lo {
			lo = txs[0]
		}
		for _, sp := range spans {
			if sp.From > 0 && sp.From < lo {
				lo = sp.From
			}
		}
		if a.Win != nil && a.Win.From < lo {
			lo = a.Win.From
		}
		if !complete {
			lo -= win // справа налево: слева ещё «неизвестная» полоса, пока не дошли до начала
		}
	}
	if hi-lo < win {
		lo = hi - win
	}
	av.AxisFrom, av.AxisTo = lo, hi

	var covered int64
	for _, sp := range spans {
		f, t := sp.From, sp.To
		if f < lo {
			f = lo
		}
		if t > hi {
			t = hi
		}
		if t > f {
			covered += t - f
		}
	}
	span := hi - lo
	pct := int(covered * 100 / span)
	switch {
	case a.State == "done" && a.Error == "":
		pct = 100
	case complete && pct > 100:
		pct = 100
	case pct > 99:
		pct = 99
	}
	if pct < 0 {
		pct = 0
	}
	av.Percent = pct

	cells := make([]byte, timelineCells)
	for i := range cells {
		ca := lo + span*int64(i)/timelineCells
		cb := lo + span*int64(i+1)/timelineCells
		mid := (ca + cb) / 2
		if _, ok := db.SpanAt(spans, mid); !ok {
			cells[i] = '0'
			continue
		}
		j := sort.Search(len(txs), func(k int) bool { return txs[k] >= ca })
		if j < len(txs) && txs[j] < cb {
			cells[i] = '1'
		} else {
			cells[i] = '2'
		}
	}
	av.Cells = string(cells)

	av.remaining = s.remainingRequests(a, spans, txs, complete, lo, hi, covered)
	return av
}

// customAccView — таймлайн источника без таблицы покрытия (Binance): ось [первое движение … сейчас], заливка — по проценту
// оценки; зелёная клетка — в ней были движения журнала, косая — не было.
func (s *Server) customAccView(a *accProg, now int64) accView {
	lo := a.AxisFrom
	if lo <= 0 || lo >= now { // начало истории ещё не известно — показываем последние 90 суток
		lo = now - 90*86400
	}
	span := now - lo
	pct := a.Pct
	if a.State == "done" {
		pct = 100
	}
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	flows := append([]int64(nil), a.FlowAt...)
	sort.Slice(flows, func(i, j int) bool { return flows[i] < flows[j] })
	cells := make([]byte, timelineCells)
	for i := range cells {
		ca, cb := lo+span*int64(i)/timelineCells, lo+span*int64(i+1)/timelineCells
		filled := i*100 < pct*timelineCells
		j := sort.Search(len(flows), func(k int) bool { return flows[k] >= ca })
		switch hasFlow := j < len(flows) && flows[j] < cb; {
		case hasFlow && filled && a.State != "queued":
			cells[i] = '1'
		case filled:
			cells[i] = '2'
		default:
			cells[i] = '0'
		}
	}
	av := accView{ID: a.ID, Label: a.Label, State: a.State, Percent: pct, AxisFrom: lo, AxisTo: now, Cells: string(cells), Error: a.Error, Note: a.Note, CoolUntil: a.CoolUntil}
	if a.State == "fetch" || a.State == "cooling" {
		av.Win = a.Win
	}
	if a.State != "done" && a.State != "error" { // сколько запросов осталось (для «закончится ≈»)
		av.remaining = a.Left
	}
	return av
}

// accLabel — подпись счёта в панели: «UA32****0965 · black».
func accLabel(a mono.AccountDTO) string {
	id := a.IBAN
	if len(id) >= 8 {
		id = id[:4] + "****" + id[len(id)-4:]
	} else if len(a.MaskedPan) > 0 {
		id = a.MaskedPan[0]
	}
	if id == "" {
		id = a.ID
	}
	if a.Type == "" {
		return id
	}
	return fmt.Sprintf("%s · %s", id, a.Type)
}

func progAccounts(list []mono.AccountDTO) []*accProg {
	out := make([]*accProg, 0, len(list))
	for _, a := range list {
		out = append(out, &accProg{ID: a.ID, Label: accLabel(a), State: "queued"})
	}
	return out
}

// remainingRequests — сколько запросов к банку этому счёту ещё предстоит (для прикидки времени).
// Считаем так же, как идёт обход: дыры по оси (каждые 30 дней — запрос), плюс «пустой хвост» —
// сколько пустых окон ещё нужно, чтобы объявить начало истории (уже снятые пустые окна засчитываются).
func (s *Server) remainingRequests(a *accProg, spans []db.Span, txs []int64, complete bool, lo, hi, covered int64) int {
	if a.State == "done" || a.State == "error" {
		return 0
	}
	if complete {
		return 1 // только хвост к «сейчас»
	}
	win := int64(walkWindow / time.Second)
	opened := s.storage.OpenedAt(a.ID)

	// нижняя граница, ниже которой дыры считаем: открытие, либо самый низ уже снятого
	floor, lowCov := lo, int64(0)
	for _, sp := range spans {
		if sp.From > 0 && (lowCov == 0 || sp.From < lowCov) {
			lowCov = sp.From
		}
	}
	switch {
	case opened > 0:
		floor = opened
	case lowCov > 0:
		floor = lowCov
	default:
		floor = hi
	}
	var gaps int64
	if hi > floor {
		var cov int64
		for _, sp := range spans {
			f, t := sp.From, sp.To
			if f < floor {
				f = floor
			}
			if t > hi {
				t = hi
			}
			if t > f {
				cov += t - f
			}
		}
		gaps = hi - floor - cov
	}
	rem := int((gaps + win - 1) / win)

	if opened == 0 {
		var emptyDone int64
		switch {
		case len(txs) > 0 && lowCov > 0 && txs[0] > lowCov:
			emptyDone = (txs[0] - lowCov) / win
		case len(txs) == 0 && lowCov > 0:
			emptyDone = covered / win
		}
		if left := walkMaxEmpty - int(emptyDone); left > 0 {
			rem += left
		}
	}
	if rem < 1 {
		rem = 1
	}
	return rem
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

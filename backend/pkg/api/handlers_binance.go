package api

import (
	"bytes"
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

// Binance — клиент «Разведать» наравне с банками: read-only ключ из .env (BINANCE_API_KEY / BINANCE_API_SECRET),
// всё полученное — кеш (см. db/binance.go). Отдельного опроса и своих кнопок нет: обновляется только через «Разведать»,
// а «Вайп кеша» сносит и Binance, после чего следующий «Разведать» восстанавливает историю с нуля.

type binanceState struct {
	client   *binance.Client
	histFrom time.Time // как далеко назад искать движения (по умолчанию 2020-01-01)
}

// SetBinance подключает клиент Binance (nil — интеграция выключена).
func (s *Server) SetBinance(c *binance.Client) { s.bn.client = c }

func (s *Server) SetBinanceHistoryFrom(t time.Time) { s.bn.histFrom = t }

// binanceFrom — как далеко назад искать движения (по умолчанию 2020-01-01).
func (s *Server) binanceFrom() time.Time {
	if s.bn.histFrom.IsZero() {
		return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return s.bn.histFrom
}

// Этапы Binance: в панели «Разведать» каждый — своя строка-таймлайн (кроме быстрых «остатков»).
var (
	rowStages = []string{"депозиты", "выводы", "конвертации", "покупки картой", "P2P", "начисления", "фьючерсы", "пыль", "стейкинг", "сделки", "цены"}
	stageName = map[string]string{
		"остатки": "Остатки и оценка в ₴", "депозиты": "Депозиты", "выводы": "Выводы", "конвертации": "Конвертации",
		"покупки картой": "Покупки картой", "P2P": "P2P", "начисления": "Начисления", "сделки": "Сделки", "цены": "Минутные цены",
		"фьючерсы": "Переводы на фьючерсы", "пыль": "Конвертация пыли", "стейкинг": "Стейкинг SOL",
		"ряд": "Минутный ряд",
	}
	// длина окна запроса у этапов, поддерживающих период (Binance ограничивает: 89 или 29 суток) и число обходов
	stageStep = map[string]struct {
		days int64
		mult int
	}{"депозиты": {89, 1}, "выводы": {89, 1}, "конвертации": {29, 1}, "покупки картой": {29, 1}, "P2P": {29, 2}, "начисления": {89, 1},
		"фьючерсы": {29, 2}, "пыль": {89, 1}, "стейкинг": {89, 2}}
)

// crawlBinance — Binance в «Разведать»: клиент в панели прогресса. Пока истории нет — строки по статьям (депозиты, выводы,
// конвертации, … сделки, минутные цены), у каждой честный таймлайн: ось — период, окно — запрашиваемый сейчас период,
// зелёные клетки — найденные движения. Каждый просмотренный период сразу пишется в кеш (db.SaveBinanceWindow), и прерванный
// синк продолжается с него. Когда ряд уже есть — одна строка «Портфель» (дописывание).
func (s *Server) crawlBinance() {
	cp := s.prog.client("Binance", "binance", 0)
	started, req0 := time.Now(), s.bn.client.Requests()
	var rows []*accProg
	byStage := map[string]*accProg{}
	incremental := false
	if first, _, n := s.storage.BinanceSeriesInfo(); n > 0 {
		incremental = true
		row := &accProg{ID: "binance", Label: "Портфель", State: "fetch", Custom: true, AxisFrom: first, Note: stageName["остатки"], Left: 2}
		var d struct {
			History struct {
				FlowAt []int64 `json:"flow_at"`
			} `json:"history"`
		}
		_ = json.Unmarshal(s.storage.BinanceDetail(), &d)
		row.FlowAt = d.History.FlowAt
		rows = []*accProg{row}
	} else {
		for _, st := range rowStages {
			r := &accProg{ID: "binance-" + st, Label: stageName[st], State: "queued", Custom: true, Ahead: st == "сделки"}
			if sp, ok := stageStep[st]; ok {
				r.Step, r.Mult = sp.days*86400, sp.mult
			}
			if t := binance.StageStart(st, s.binanceFrom(), time.Now()); !t.IsZero() { // Binance не отдаёт историю глубже — ось честная
				r.AxisFrom = t.Unix()
			}
			rows, byStage[st] = append(rows, r), r
		}
	}
	s.prog.setAccounts(cp, rows)

	// средняя длительность запроса — для оценки «закончится ≈»
	pace := func() {
		if n := s.bn.client.Requests() - req0; n >= 3 {
			sec := time.Since(started).Seconds() / float64(n)
			s.prog.mu.Lock()
			cp.ReqSec = sec
			s.prog.mu.Unlock()
		}
	}
	// диагностика: сколько ждали по лимиту и какие периоды отвечали долго (видно в консоли бэкенда)
	var waits int
	var slowest time.Duration
	var t0 time.Time
	var cur string
	var pd, pt int                               // монеты цен: готово из скольки
	pricesSpan := func(a *accProg, from int64) { // доля текущей монеты по времени → общий процент цен
		if pt > 0 && a.AxisFrom > 0 {
			if span := time.Now().Unix() - a.AxisFrom; span > 0 {
				frac := float64(from/1000-a.AxisFrom) / float64(span)
				if frac < 0 {
					frac = 0
				} else if frac > 1 {
					frac = 1
				}
				a.Pct = int((float64(pd) + frac) * 100 / float64(pt))
			}
		}
	}
	hk := &binanceHooks{
		Wait: func(d time.Duration) { // лимит Binance: «остывание», как у банков
			if d > 0 {
				waits++
				s.updateStatus(fmt.Sprintf("[Binance] ЛИМИТ ЗАПРОСОВ · ОХЛАЖДЕНИЕ %d С", int(d.Seconds())))
			}
			for _, r := range rows {
				s.prog.with(r, func(a *accProg) {
					switch {
					case d > 0 && a.State == "fetch":
						a.State, a.CoolUntil = "cooling", time.Now().Add(d).Unix()
					case d == 0 && a.State == "cooling":
						a.State, a.CoolUntil = "fetch", 0
					}
				})
			}
		},
	}
	if incremental {
		hk.Stage = func(stage string, _, _ int) {
			s.updateStatus("[Binance] " + strings.ToUpper(stageName[stage]))
			s.prog.with(rows[0], func(a *accProg) { a.Note = stageName[stage] })
			pace()
		}
	} else {
		hk.Stage = func(stage string, d, t int) {
			row := byStage[stage]
			if row == nil { // остатки: быстро, отдельной строки нет
				if cur != stage {
					cur = stage
					s.updateStatus("[Binance] " + strings.ToUpper(stageName[stage]))
				}
				return
			}
			if stage != cur { // новый этап: прежние готовы, этот качается
				cur = stage
				s.updateStatus("[Binance] " + strings.ToUpper(stageName[stage]))
				for _, st := range rowStages {
					if st == stage {
						break
					}
					s.prog.with(byStage[st], func(a *accProg) { a.State, a.Pct, a.Note, a.Win, a.Left, a.Cov = "done", 100, "", nil, 0, 0 })
				}
				s.prog.with(row, func(a *accProg) { a.State = "fetch" })
			}
			switch stage {
			case "сделки", "цены":
				unit := map[string]string{"сделки": "пар", "цены": "монет"}[stage]
				if stage == "цены" {
					pd, pt = d, t
				}
				if t > 0 {
					s.prog.with(row, func(a *accProg) {
						a.Pct, a.Note, a.Left = d*100/t, fmt.Sprintf("%s %d/%d", unit, d, t), t-d
						if stage == "цены" && a.AxisFrom > 0 { // каждая оставшаяся монета — свои куски по 1000 минут
							a.Left = (t - d) * int((time.Now().Unix()-a.AxisFrom)/60/1000+1)
						}
					})
				}
				if stage == "сделки" && d%100 == 0 && d > 0 {
					s.updateStatus(fmt.Sprintf("[Binance] СДЕЛКИ %d/%d", d, t))
				}
			}
			pace()
		}
		hk.Start = func(stage string, from, to int64) { // запрашивается именно этот период
			t0 = time.Now()
			row := byStage[stage]
			if row == nil {
				return
			}
			s.prog.with(row, func(a *accProg) {
				a.Win = &progWin{From: from / 1000, To: to / 1000}
				if stage == "цены" {
					pricesSpan(a, from)
				} else if _, windowed := stageStep[stage]; windowed {
					a.Cov = to / 1000 // справа налево: просмотрено всё, что правее запрашиваемого окна
				}
			})
		}
		hk.Done = func(stage, _ string, from, to int64, flows []int64, cached bool) {
			if d := time.Since(t0); !cached && !t0.IsZero() {
				if d > slowest {
					slowest = d
				}
				if d >= 3*time.Second && stage != "цены" {
					log.Printf("[Sync] Binance: %s %s — %s отвечал %s", stageName[stage], time.Unix(from/1000, 0).Format("2006-01-02"), time.Unix(to/1000, 0).Format("2006-01-02"), d.Round(100*time.Millisecond))
				}
			}
			row := byStage[stage]
			if row == nil {
				return
			}
			s.prog.with(row, func(a *accProg) {
				if _, windowed := stageStep[stage]; windowed { // сделки идут по парам, а не по времени: «просмотрено до» у них нет
					a.Cov = from / 1000 // окно обработано: просмотрено от его начала до сегодня
				}
				a.FlowAt = append(a.FlowAt, secs(flows)...)
			})
			pace()
		}
		hk.Period = func(first int64, flows []int64) { // реальная история найдена: ось минутных цен — от первого движения
			s.prog.with(byStage["цены"], func(a *accProg) { a.AxisFrom, a.FlowAt = first/1000, secs(flows) })
		}
	}

	err := s.syncBinance(hk)
	log.Printf("[Sync] Binance: запросов %d за %s, пауз по лимиту %d, самый долгий период %s",
		s.bn.client.Requests()-req0, time.Since(started).Round(time.Second), waits, slowest.Round(100*time.Millisecond))
	for _, r := range rows {
		s.prog.with(r, func(a *accProg) {
			a.Note, a.Win, a.CoolUntil, a.Left = "", nil, 0, 0
			switch {
			case err != nil && (a.State == "fetch" || a.State == "cooling"):
				a.State, a.Error = "error", err.Error()
			case err == nil:
				a.State, a.Pct = "done", 100
			}
		})
	}
	if err != nil {
		log.Printf("[Sync Error] Binance: %v", err)
		s.updateStatus("[Binance] СБОЙ: " + err.Error())
	}
}

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

// binanceHooks — куда сообщать ход синка (панель «Разведать»); любое поле можно не задавать.
type binanceHooks struct {
	Stage  func(stage string, done, total int)
	Period func(firstMs int64, flowsMs []int64)
	Start  func(stage string, fromMs, toMs int64)                                    // период запрашивается сейчас
	Done   func(stage, key string, fromMs, toMs int64, flowsMs []int64, cached bool) // период обработан
	Wait   func(d time.Duration)                                                     // пауза из-за лимита
}

// syncBinance: остатки → оценка в ₴ → минутный ряд (с нуля, если в кеше его нет; иначе дописать до сейчас) → кеш.
func (s *Server) syncBinance(hk *binanceHooks) error {
	c := s.bn.client
	if c == nil {
		return errors.New("Binance не подключён: нет BINANCE_API_KEY / BINANCE_API_SECRET")
	}
	if hk == nil {
		hk = &binanceHooks{}
	}
	progress := func(stage string, d, t int) {
		if hk.Stage != nil {
			hk.Stage(stage, d, t)
		}
	}
	progress("остатки", 0, 0)
	h, warns, err := c.Fetch()
	if err != nil {
		return err
	}
	prices, err := c.Prices()
	if err != nil {
		return err
	}
	v, ok := binance.Value(h, prices)
	if !ok {
		return errors.New("нет курса USDT/UAH — оценить портфель в гривнах нечем")
	}

	now := time.Now()
	detail := map[string]any{
		"taken_at": now.Unix(), "total": v.TotalUAH, "usdt_uah": v.USDTUAH,
		"items": v.Items, "unpriced": v.Unpriced,
	}
	var (
		start   int64
		vals    []float64
		replace bool
		origin  float64 // оценка «до первой точки»: 0 — счёт начался с первого найденного движения
	)
	var prevHist any
	{
		var prev struct {
			History any `json:"history"`
		}
		_ = json.Unmarshal(s.storage.BinanceDetail(), &prev)
		prevHist = prev.History
	}
	// самым первым шагом — счёт и сегодняшняя оценка в кеш: он виден в «Операционной», пока история ещё собирается
	early := map[string]any{}
	for k, v := range detail {
		early[k] = v
	}
	early["history"], early["warnings"] = prevHist, warns
	if err := s.storage.SaveBinanceAccount(v.TotalUAH, early); err != nil {
		return err
	}
	s.valueBinanceTxs(c) // допишем операции, оценка которых не успела сохраниться в прошлый раз
	if last, _, have := s.storage.BinanceLastMinute(); have {
		progress("ряд", 0, 0)
		res, err := c.ExtendSeries(h, last*1000, now.UnixMilli())
		if err != nil {
			warns = append(warns, "минутный ряд: "+err.Error())
		} else if len(res.Values) > 0 {
			start, vals = res.StartMs/1000, res.Values
		}
		detail["history"] = prevHist
	} else {
		cached, err := s.storage.BinanceFlows()
		if err != nil {
			return err
		}
		opts := binance.HistoryOpts{
			From: s.binanceFrom(), Now: now, Progress: progress, Period: hk.Period, Start: hk.Start, Wait: hk.Wait,
			Cached:  toBinanceFlows(cached),
			Covered: s.storage.BinanceCovered,
			Done: func(stage, key string, from, to int64, fl []binance.Flow, isCached bool) {
				if hk.Done != nil {
					ts := make([]int64, len(fl))
					for i, f := range fl {
						ts[i] = f.T
					}
					hk.Done(stage, key, from, to, ts, isCached)
				}
				if isCached || stage == "цены" {
					return
				}
				// каждый просмотренный период сразу в кеш: прерванный синк продолжится отсюда
				if err := s.storage.SaveBinanceWindow(stage, key, from, to, fromBinanceFlows(fl)); err != nil {
					log.Printf("[Sync Error] Binance: кеш периода %s: %v", stage, err)
				}
				if stage != "сделки" {
					s.valueBinanceTxs(c) // депозиты/выводы/… сразу становятся операциями счёта
				}
			},
		}
		res, err := c.BuildHistory(opts)
		if err != nil {
			return err
		}
		warns = append(warns, res.Warnings...)
		replace = true
		if res.Series != nil && len(res.Series.Values) > 0 {
			start, vals = res.Series.StartMs/1000, res.Series.Values
		} else { // движений не нашлось — ряд начинается с сегодняшней оценки
			start, vals, origin = now.Truncate(time.Minute).Unix(), []float64{v.TotalUAH}, v.TotalUAH
		}
		detail["history"] = map[string]any{"flows": res.Flows, "residual": res.Residual, "at": now.Unix(), "flow_at": secs(res.FlowTimes)}
	}
	detail["warnings"] = warns
	if err := s.storage.SaveBinance(db.BinanceSave{Portfolio: v.TotalUAH, Start: start, Vals: vals, Replace: replace, Origin: origin, Detail: detail}); err != nil {
		return err
	}
	for _, w := range warns {
		log.Printf("[Sync] Binance: %s", w)
	}
	first, last, n := s.storage.BinanceSeriesInfo()
	log.Printf("[Sync] Binance: портфель %.2f ₴, ряд %s — %s (%d мин), история %s", v.TotalUAH,
		time.Unix(first, 0).Format("2006-01-02"), time.Unix(last, 0).Format("2006-01-02 15:04"), n, map[bool]string{true: "собрана с нуля", false: "дописана"}[replace])
	return nil
}

// valueBinanceTxs оценивает в ₴ внешние движения, которых ещё нет среди операций счёта, и сразу кладёт их в кеш.
func (s *Server) valueBinanceTxs(c *binance.Client) {
	rows, err := s.storage.BinanceUnvaluedFlows(binance.ExtKinds())
	if err != nil {
		log.Printf("[Sync Error] Binance: операции счёта: %v", err)
		return
	}
	var txs []db.BinanceTx
	for _, r := range rows {
		lbl, _ := binance.ExtLabel(r.Kind)
		if r.Kind == "strategy" { // у перевода есть направление
			lbl = map[bool]string{true: "возврат из Strategy", false: "перевод в Strategy"}[r.Delta > 0]
		}
		uah, err := c.ValueUAH(r.Asset, r.Delta, r.T)
		if err != nil {
			log.Printf("[Sync] Binance: %s %s — оценить в ₴ не вышло: %v", lbl, r.Asset, err)
		}
		qty := strconv.FormatFloat(math.Abs(r.Delta), 'f', -1, 64)
		txs = append(txs, db.BinanceTx{FlowID: r.ID, T: r.T, Desc: fmt.Sprintf("Binance: %s %s %s", lbl, qty, r.Asset), UAH: uah})
	}
	if err := s.storage.SaveBinanceTxs(txs); err != nil {
		log.Printf("[Sync Error] Binance: операции счёта: %v", err)
	}
}

// importBinanceStatement — выписка Binance в общем импорте («Дебаг → Импорт», «Загрузить»): движения, которых нет в API
// (Strategy, фьючерсы, стейкинг, пыль, кэшбэк), попадают в кеш движений рядом с API-шными; ряд пересоберётся при «Разведать».
func (s *Server) importBinanceStatement(filename string, raw []byte) (*importResult, error) {
	flows, _, err := binance.ParseStatement(bytes.NewReader(raw), binance.StatementZone(filename))
	if err != nil {
		return nil, &importError{http.StatusBadRequest, "Failed to parse Binance statement: " + err.Error()}
	}
	if len(flows) > 0 {
		from, to := flows[0].T, flows[0].T
		for _, f := range flows {
			from, to = min(from, f.T), max(to, f.T)
		}
		if err := s.storage.SaveBinanceWindow("выписка", contentHash(raw), from, to, fromBinanceFlows(flows)); err != nil {
			return nil, &importError{http.StatusInternalServerError, "Failed to save Binance statement: " + err.Error()}
		}
		if err := s.storage.DropBinanceSeries(); err != nil {
			return nil, &importError{http.StatusInternalServerError, "Failed to reset Binance series: " + err.Error()}
		}
	}
	return &importResult{AccountID: db.BinanceAccountID, Card: "binance", Count: len(flows)}, nil
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
		"configured": s.bn.client != nil, "detail": s.storage.BinanceDetail(),
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

func toBinanceFlows(in []db.BinanceFlow) []binance.Flow {
	out := make([]binance.Flow, len(in))
	for i, f := range in {
		out[i] = binance.Flow{T: f.T, Asset: f.Asset, Delta: f.Delta, Kind: f.Kind}
	}
	return out
}

func fromBinanceFlows(in []binance.Flow) []db.BinanceFlow {
	out := make([]db.BinanceFlow, len(in))
	for i, f := range in {
		out[i] = db.BinanceFlow{T: f.T, Asset: f.Asset, Delta: f.Delta, Kind: f.Kind}
	}
	return out
}

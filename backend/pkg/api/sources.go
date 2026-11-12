package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"economic/pkg/db"
	"economic/pkg/source"
)

// ───────────── Источники ─────────────
//
// Это НЕ кэш-база (cache.db). Это папка на диске с файлами-источниками формата .zoe (пакет source):
//
//	~/.local/share/economic/sources/<PAN с маской | binance | icu>/<дата-начала>_<отпечаток>.zoe
//
// • в файле уже разобранные данные, а не исходный PDF/CSV; имя исходного файла (алиас) пишется один раз, при первом импорте;
// • дубль определяется по отпечатку СОДЕРЖИМОГО, а не по имени;
// • папка = номер карты (не IBAN: один IBAN может нести два PAN), посередине звёздочки: 516875******8629;
// • сброс кэша-базы источники не трогает, «Загрузить» кладёт их обратно в базу.
//
// Папку можно поменять переменной ECONOMIC_SOURCES_DIR.

func sourcesDir() string {
	if d := strings.TrimSpace(os.Getenv("ECONOMIC_SOURCES_DIR")); d != "" {
		return d
	}
	if x := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); x != "" {
		return filepath.Join(x, "economic", "sources")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "economic", "sources")
	}
	return filepath.Join(home, ".local", "share", "economic", "sources")
}

func digitsOf(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// maskPAN — имя папки: первые 6 цифр + звёздочки + последние 4 (414951******5262).
// Если карта пришла в другой маске (4149****7777) — остаётся как есть.
func maskPAN(card string) string {
	card = strings.TrimSpace(card)
	if card == "" {
		return "unknown"
	}
	if card == "binance" || card == "icu" { // выписка Binance и брокерский отчёт ICU — не карты: своя папка
		return card
	}
	var out string
	if i := strings.IndexAny(card, "*•xX"); i >= 0 {
		lead, tail := digitsOf(card[:i]), digitsOf(card[i:])
		if len(lead) >= 6 && len(tail) >= 4 {
			out = lead[:6] + "******" + tail[len(tail)-4:]
		} else {
			out = lead + "****" + tail
		}
	} else if d := digitsOf(card); len(d) >= 12 {
		out = d[:6] + strings.Repeat("*", len(d)-10) + d[len(d)-4:]
	} else {
		out = strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' || r == '*' {
				return r
			}
			return '_'
		}, card)
	}
	if out == "" {
		return "unknown"
	}
	return out
}

type loadFailure struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

// accountsReq — тело запросов «Загрузить»/«Сбросить»: пусто = все счета, иначе только перечисленные.
type accountsReq struct {
	Accounts []string `json:"accounts"`
}

func readAccounts(r *http.Request) map[string]bool {
	var in accountsReq
	_ = json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&in)
	if len(in.Accounts) == 0 {
		return nil
	}
	out := map[string]bool{}
	for _, id := range in.Accounts {
		out[id] = true
	}
	return out
}

// HandleLoadSources — «Загрузить»: файлы-источники кладутся в базу (все или только выбранных счетов).
func (s *Server) HandleLoadSources(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}
	want := readAccounts(r)
	_, failed := migrateLegacy()
	entries, bad := store().List()
	for p, why := range bad {
		failed = append(failed, loadFailure{filepath.Base(p), why})
	}

	sort.SliceStable(entries, func(i, j int) bool { // ряд Binance — после выписки (он строится по её журналу, а импорт выписки ряд сбрасывает)
		return entries[i].File.Kind != source.KindSeries && entries[j].File.Kind == source.KindSeries
	})

	total, loaded, txCount := 0, 0, 0
	for _, e := range entries {
		if want != nil && !want[e.File.AccountID] {
			continue
		}
		total++
		res, err := s.applySource(e.File)
		if err != nil {
			log.Printf("[Sources] %s: %v", e.Path, err)
			failed = append(failed, loadFailure{e.File.Name, err.Error()})
			continue
		}
		loaded++
		txCount += res.Count
	}
	total += len(failed)
	if failed == nil {
		failed = []loadFailure{}
	}

	writeJSON(w, map[string]interface{}{
		"status": "success", "total": total, "loaded": loaded, "transactions": txCount, "failed": failed, "dir": sourcesDir(),
	})
}

// HandleResetSources — «Сбросить»: кеш-база очищается (вся или у выбранных счетов). Файлы-источники не трогаются.
func (s *Server) HandleResetSources(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}
	want := readAccounts(r)
	s.abortSync() // идущая «Разведка» дописала бы данные в только что очищенное
	var err error
	if want == nil {
		err = s.storage.WipeCache()
	} else {
		ids := make([]string, 0, len(want))
		for id := range want {
			ids = append(ids, id)
		}
		err = s.storage.WipeAccounts(ids)
	}
	if err != nil {
		failErr(w, err)
		return
	}
	writeOK(w)
}

// ───────────── Инвентарь: что известно об источниках ─────────────

const invCells = 96

type invFile struct {
	ID      string `json:"id"` // отпечаток содержимого
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Created int64  `json:"created"`
	Origin  string `json:"origin"`
	From    int64  `json:"from"`
	To      int64  `json:"to"`
	Count   int    `json:"count"`
}

type invAccount struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Label    string    `json:"label"` // IBAN / «Binance» / договор — как опознать счёт
	Loaded   bool      `json:"loaded"`
	Cached   int       `json:"cached"`  // записей в кеш-базе
	Records  int       `json:"records"` // записей во всех источниках счёта
	Files    []invFile `json:"files"`
	From     int64     `json:"from"` // что покрыто: от … до
	To       int64     `json:"to"`
	AxisFrom int64     `json:"axis_from"`
	AxisTo   int64     `json:"axis_to"`
	Cells    string    `json:"cells"` // 0 — данных нет, 1 — покрыто, операции есть, 2 — покрыто, операций не было
}

// nameRank: файл от человека (имя = псевдоним владельца) важнее служебного снимка API и ряда минут.
func nameRank(f *source.File) int {
	if f.Origin == source.OriginAPI || f.Kind == source.KindSeries {
		return 0
	}
	return 1
}

// coverCells раскрашивает ось: нет заливки / зелёный / косые полосы — как таймлайн синхронизации.
func coverCells(spans []db.Span, txs []int64, lo, hi int64) string {
	if hi <= lo {
		hi = lo + 1
	}
	span := hi - lo
	cells := make([]byte, invCells)
	for i := range cells {
		ca, cb := lo+span*int64(i)/invCells, lo+span*int64(i+1)/invCells
		covered := false
		for _, sp := range spans { // клетка закрашена, если её касается хоть один снятый отрезок (короткий отрезок не должен пропасть)
			if sp.From < cb && sp.To >= ca {
				covered = true
				break
			}
		}
		if !covered {
			cells[i] = '0'
			continue
		}
		cells[i] = txCell(txs, ca, cb)
	}
	return string(cells)
}

func mergeSpans(in []db.Span) []db.Span {
	sort.Slice(in, func(i, j int) bool { return in[i].From < in[j].From })
	var out []db.Span
	for _, sp := range in {
		if n := len(out); n > 0 && sp.From <= out[n-1].To+1 {
			if sp.To > out[n-1].To {
				out[n-1].To = sp.To
			}
			continue
		}
		out = append(out, sp)
	}
	return out
}

// HandleSources — GET /api/sources: все счета, у которых есть источник или данные в кеше, со сведениями о файлах и таймлайном.
func (s *Server) HandleSources(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	migrated, failed := migrateLegacy()
	entries, bad := store().List()
	for p, why := range bad {
		failed = append(failed, loadFailure{filepath.Base(p), why})
	}
	counts, _ := s.storage.CacheCounts()
	cached, _ := s.storage.GetAccounts()

	byID := map[string]*invAccount{}
	order := []string{}
	spans := map[string][]db.Span{}
	times := map[string][]int64{}
	nameRanks := map[string]int{}
	get := func(id string) *invAccount {
		if a, ok := byID[id]; ok {
			return a
		}
		a := &invAccount{ID: id, Files: []invFile{}}
		byID[id] = a
		order = append(order, id)
		return a
	}
	for _, e := range entries {
		f := e.File
		a := get(f.AccountID)
		// имя счёта — имя файла, который дал человек (csv/pdf/отчёт); у снимка API и ряда минут имя служебное («Binance»), оно лишь запасное
		if rank := nameRank(f); a.Name == "" || rank > nameRanks[f.AccountID] {
			a.Name, nameRanks[f.AccountID] = f.Name, rank
		}
		a.Type = f.Kind
		if f.Account != nil {
			a.Type = f.Account.Type
			if f.Account.IBAN != "" {
				a.Label = f.Account.IBAN
			}
		}
		if a.Label == "" {
			a.Label = f.Card
		}
		a.Files = append(a.Files, invFile{ID: f.Hash, Kind: f.Kind, Name: f.Name, Created: f.Created, Origin: f.Origin, From: f.From, To: f.To, Count: f.Count()})
		a.Records += f.Count()
		if from, to := f.Cover(); to > 0 {
			spans[f.AccountID] = append(spans[f.AccountID], db.Span{From: from, To: to})
		}
		for _, t := range f.Txs {
			times[f.AccountID] = append(times[f.AccountID], t.Time)
		}
		for _, r := range f.Rows {
			times[f.AccountID] = append(times[f.AccountID], r.T/1000)
		}
		for _, fl := range f.Flows {
			times[f.AccountID] = append(times[f.AccountID], fl.Time)
		}
	}
	for _, c := range cached {
		a := get(c.ID)
		a.Name, a.Type = c.Name, c.Type
		if c.IBAN != "" {
			a.Label = c.IBAN
		} else if a.Label == "" {
			a.Label = c.MaskedPan
		}
		if cov, err := s.storage.Coverage(c.ID); err == nil { // снятое с API тоже считается покрытием
			spans[c.ID] = append(spans[c.ID], cov...)
		}
		if ts, err := s.storage.TxTimes(c.ID); err == nil {
			times[c.ID] = append(times[c.ID], ts...)
		}
	}

	now := time.Now().Unix()
	out := make([]*invAccount, 0, len(order))
	for _, id := range order {
		a := byID[id]
		a.Cached = counts[id]
		a.Loaded = a.Cached > 0
		sp := mergeSpans(spans[id])
		ts := times[id]
		sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
		lo, hi := now, now
		if len(sp) > 0 {
			a.From, a.To = sp[0].From, sp[len(sp)-1].To
			if sp[0].From > db.HistoryStart && sp[0].From < lo {
				lo = sp[0].From
			}
		}
		if len(ts) > 0 && ts[0] < lo {
			lo = ts[0]
		}
		a.AxisFrom, a.AxisTo = lo, hi
		a.Cells = coverCells(sp, ts, lo, hi)
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	writeJSON(w, map[string]interface{}{"dir": sourcesDir(), "accounts": out, "migrated": migrated, "failed": failedOrEmpty(failed)})
}

func failedOrEmpty(f []loadFailure) []loadFailure {
	if f == nil {
		return []loadFailure{}
	}
	return f
}

// HandleSaveAPI — «Сохранить с API»: то, что уже снято с API (операции банков с отрезками покрытия, минутный ряд Binance), пишется
// в файлы-источники. Сброс кеша потом восстанавливается с диска, а снятые отрезки заново не качаются.
// Тело: {"accounts":[…]}; пусто — все счета, у которых есть что сохранить.
func (s *Server) HandleSaveAPI(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}
	out, err := s.saveAPISources(readAccounts(r))
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"status": "success", "results": out})
}

// saveRes — итог записи одного счёта в файлы-источники.
type saveRes struct {
	Account string `json:"account"`
	Name    string `json:"name"`
	Saved   bool   `json:"saved"`
	Skipped string `json:"skipped,omitempty"`
	Count   int    `json:"count"`
	Error   string `json:"error,omitempty"`
}

// saveAPISources пишет снятое с API в файлы-источники. want == nil — все счета, у которых есть что сохранять
// (счета без данных API молча пропускаются); иначе только перечисленные, и «нечего сохранять» попадает в итог.
// Тот же путь, что у кнопки «Сохранить с API» и у автозаписи после «Разведать».
func (s *Server) saveAPISources(want map[string]bool) ([]saveRes, error) {
	accs, err := s.storage.GetAccounts()
	if err != nil {
		return nil, err
	}
	out := []saveRes{}
	for _, a := range accs {
		if want != nil && !want[a.ID] {
			continue
		}
		f, err := s.snapshotAPI(a)
		item := saveRes{Account: a.ID, Name: a.Name}
		switch {
		case err != nil:
			item.Error = err.Error()
		case f == nil:
			if want == nil {
				continue // «все»: счета без данных API молча пропускаем
			}
			item.Skipped = "с API по этому счёту ничего не снято"
		default:
			_, added, perr := store().PutReplacing(f)
			if perr != nil {
				item.Error = perr.Error()
			} else {
				item.Saved, item.Count = added, f.Count()
				if !added {
					item.Skipped = "источник уже актуален"
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// HandleDeleteSource — DELETE /api/sources/file?id=<отпечаток>: снести файл-источник.
// Если это снимок API, кеш забывает, что отрезок снят (и сбрасывает ряд Binance): следующая синхронизация стянет всё с API заново.
func (s *Server) HandleDeleteSource(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodDelete) {
		return
	}
	e, ok := store().Find(r.URL.Query().Get("id"))
	if !ok {
		fail(w, http.StatusNotFound, "источник не найден")
		return
	}
	if err := store().Remove(e.Path); err != nil {
		failErr(w, err)
		return
	}
	refetch := false
	if e.File.Origin == source.OriginAPI {
		s.abortSync()
		var err error
		if e.File.Kind == source.KindSeries {
			err = s.storage.DropBinanceSeries()
		} else {
			err = s.storage.ClearCoverage(e.File.AccountID)
		}
		if err != nil {
			failErr(w, err)
			return
		}
		refetch = true
	}
	writeJSON(w, map[string]any{"status": "success", "refetch": refetch})
}

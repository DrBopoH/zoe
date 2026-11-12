// Package source — единый файл-формат источника данных (.zoe).
//
// Один файл = один импорт (или один снимок с API) одного счёта за один период. Формат один для всех сценариев
// (CSV, PDF, брокерский отчёт, выписка Binance, API) и всех источников (банки, брокер, Binance): внутри уже разобранные
// данные, а не исходный файл, поэтому «Загрузить» не заново парсит PDF/CSV, а просто кладёт готовое в базу.
//
//	Name    — имя исходного файла (= алиас владельца). Задаётся ОДИН раз, при первом импорте; потом не меняется.
//	Created — когда файл-источник создан.
//	Hash    — отпечаток содержимого (без Name/Created/Origin): одинаковые данные под другим именем — дубль.
package source

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"economic/pkg/mono"
)

const gzipOver = 1 << 20

const (
	Ext     = ".zoe"
	Format  = "zoe-source"
	Version = 1

	KindBank    = "bank"
	KindBinance = "binance"
	KindICU     = "icu"
	KindSeries  = "binance-series" // минутный ряд оценки портфеля Binance: данные API, собираются 10–20 минут

	OriginCSV     = "csv"
	OriginPDF     = "pdf"
	OriginReport  = "report"  // брокерский отчёт
	OriginBinance = "binance" // выписка Binance
	OriginAPI     = "api"     // снимок из API
)

// BNRow — строка журнала Binance как в выписке (вид операции пересчитывается при загрузке по текущим правилам).
type BNRow struct {
	T       int64   `json:"t"` // мс
	Account string  `json:"account"`
	Op      string  `json:"op"`
	Asset   string  `json:"asset"`
	Delta   float64 `json:"delta"`
	Note    string  `json:"note,omitempty"`
}

// Flow — запись журнала брокерского отчёта.
type Flow struct {
	ID     string  `json:"id"`
	Time   int64   `json:"time"`
	Kind   string  `json:"kind"`
	Amount float64 `json:"amount"`
	Note   string  `json:"note,omitempty"`
	ISIN   string  `json:"isin,omitempty"`
	Qty    float64 `json:"qty,omitempty"`
}

// Series — минутный ряд оценки портфеля Binance (непрерывный: точка за минутой) и то, что с ним связано.
type Series struct {
	Start     int64           `json:"start"` // unix-секунды первой минуты
	Vals      []float64       `json:"vals"`
	Origin    float64         `json:"origin"` // оценка «до первой точки»
	HasOrigin bool            `json:"has_origin"`
	Portfolio float64         `json:"portfolio"` // баланс счёта: последняя точка ряда
	Sig       string          `json:"sig"`       // подпись журнала, для которого ряд построен: журнал сменился → ряд устарел
	Detail    json.RawMessage `json:"detail,omitempty"`
}

type File struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Name    string `json:"name"`    // имя исходного файла = алиас владельца
	Created int64  `json:"created"` // unix
	Origin  string `json:"origin"`
	Kind    string `json:"kind"`
	Hash    string `json:"hash"`

	AccountID string `json:"account_id"`
	Card      string `json:"card"` // ключ папки: PAN карты, «binance» или «icu»
	From      int64  `json:"from"` // период данных по записям, unix-секунды
	To        int64  `json:"to"`
	// период, который сам источник заявляет (у брокерского отчёта он в шапке); у банков 0 — тогда «снято» кончается на свежей операции
	PeriodFrom int64 `json:"period_from,omitempty"`
	PeriodTo   int64 `json:"period_to,omitempty"`

	// отрезки, уже снятые с API: после загрузки источника синхронизация не опрашивает их заново
	Spans [][2]int64 `json:"spans,omitempty"`

	Series *Series `json:"series,omitempty"` // только у KindSeries

	Account *mono.AccountDTO        `json:"account,omitempty"`
	Txs     []mono.StatementItemDTO `json:"txs,omitempty"`

	Profile string  `json:"profile,omitempty"` // Binance: идентификатор профиля
	Rows    []BNRow `json:"rows,omitempty"`

	Contract     string  `json:"contract,omitempty"` // ICU
	ContractDate int64   `json:"contract_date,omitempty"`
	CashEnd      float64 `json:"cash_end,omitempty"`
	Flows        []Flow  `json:"flows,omitempty"`
}

// Seal проставляет служебные поля: формат, период, отпечаток. Вызывать перед записью.
func (f *File) Seal() {
	f.Format, f.Version = Format, Version
	if f.Created == 0 {
		f.Created = time.Now().Unix()
	}
	f.From, f.To = f.span()
	f.Hash = f.fingerprint()
}

// span — период данных по самим записям.
func (f *File) span() (from, to int64) {
	note := func(t int64) {
		if t <= 0 {
			return
		}
		if from == 0 || t < from {
			from = t
		}
		if t > to {
			to = t
		}
	}
	for _, t := range f.Txs {
		note(t.Time)
	}
	for _, r := range f.Rows {
		note(r.T / 1000)
	}
	for _, fl := range f.Flows {
		note(fl.Time)
	}
	if sr := f.Series; sr != nil && len(sr.Vals) > 0 {
		note(sr.Start)
		note(sr.Start + int64(len(sr.Vals)-1)*60)
	}
	return
}

// Cover — отрезок времени, который файл покрывает: период источника, а если его нет — от первой до свежей операции.
func (f *File) Cover() (from, to int64) {
	from, to = f.From, f.To
	if f.PeriodFrom > 0 && (from == 0 || f.PeriodFrom < from) {
		from = f.PeriodFrom
	}
	if f.PeriodTo > to {
		to = f.PeriodTo
	}
	for _, sp := range f.Spans {
		if sp[0] > 0 && (from == 0 || sp[0] < from) {
			from = sp[0]
		}
		if sp[1] > to {
			to = sp[1]
		}
	}
	return
}

// Count — сколько записей в файле.
func (f *File) Count() int {
	n := len(f.Txs) + len(f.Rows) + len(f.Flows)
	if f.Series != nil {
		n += len(f.Series.Vals)
	}
	return n
}

func (f *File) fingerprint() string {
	c := *f
	c.Name, c.Created, c.Origin, c.Hash, c.From, c.To = "", 0, "", "", 0, 0
	b, _ := json.Marshal(&c)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

func (f *File) Validate() error {
	if f.Format != Format {
		return errors.New("не файл-источник Ζωή")
	}
	if f.Version > Version {
		return fmt.Errorf("файл новее программы (формат %d)", f.Version)
	}
	if f.AccountID == "" || f.Kind == "" {
		return errors.New("в файле нет счёта")
	}
	return nil
}

// Entry — файл на диске.
type Entry struct {
	Path string
	File *File
}

// Store — папка с источниками: <dir>/<Card>/<дата-начала>_<hash>.zoe. Папки с «_» в начале служебные (старые файлы).
type Store struct{ Dir string }

func (s Store) path(f *File) string {
	day := "0000-00-00"
	if f.From > 0 {
		day = time.Unix(f.From, 0).UTC().Format("2006-01-02")
	}
	return filepath.Join(s.Dir, folder(f.Card), day+"_"+f.Hash+Ext)
}

func folder(card string) string {
	card = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '_'
		}
		return r
	}, strings.TrimSpace(card))
	if card == "" || card == "." || card == ".." {
		return "unknown"
	}
	return card
}

// Put записывает файл, если таких данных (по отпечатку) в хранилище ещё нет. false — это дубль.
func (s Store) Put(f *File) (string, bool, error) {
	f.Seal()
	if err := f.Validate(); err != nil {
		return "", false, err
	}
	if m, _ := filepath.Glob(filepath.Join(s.Dir, "*", "*_"+f.Hash+Ext)); len(m) > 0 {
		return m[0], false, nil
	}
	dst := s.path(f)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", false, err
	}
	b, err := json.Marshal(f)
	if err != nil {
		return "", false, err
	}
	if len(b) > gzipOver { // ряд Binance — миллионы цифр: сжимаем (Read понимает оба вида)
		var z bytes.Buffer
		zw := gzip.NewWriter(&z)
		zw.Write(b)
		zw.Close()
		b = z.Bytes()
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil { // персональные данные: доступ только владельцу
		return "", false, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", false, err
	}
	return dst, true, nil
}

// Read читает один файл.
func Read(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) > 2 && b[0] == 0x1f && b[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		if b, err = io.ReadAll(zr); err != nil {
			return nil, err
		}
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return &f, f.Validate()
}

// List — все файлы-источники; нечитаемые возвращаются во втором списке (имя → причина). Порядок: счёт, начало периода.
func (s Store) List() ([]Entry, map[string]string) {
	var out []Entry
	bad := map[string]string{}
	dirs, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, bad
	}
	for _, d := range dirs {
		if !d.IsDir() || strings.HasPrefix(d.Name(), "_") {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(s.Dir, d.Name()))
		for _, e := range files {
			if e.IsDir() || !strings.HasSuffix(e.Name(), Ext) {
				continue
			}
			p := filepath.Join(s.Dir, d.Name(), e.Name())
			f, err := Read(p)
			if err != nil {
				bad[p] = err.Error()
				continue
			}
			out = append(out, Entry{p, f})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].File, out[j].File
		if a.AccountID != b.AccountID {
			return a.AccountID < b.AccountID
		}
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return out[i].Path < out[j].Path
	})
	return out, bad
}

// Remove удаляет файл-источник.
func (s Store) Remove(path string) error {
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(s.Dir)+string(filepath.Separator)) || !strings.HasSuffix(path, Ext) {
		return errors.New("не файл хранилища")
	}
	return os.Remove(path)
}

// PutReplacing — для снимков API: у счёта хранится один актуальный снимок, прежний (того же вида) заменяется новым.
func (s Store) PutReplacing(f *File) (string, bool, error) {
	f.Seal()
	if err := f.Validate(); err != nil {
		return "", false, err
	}
	list, _ := s.List()
	for _, e := range list {
		if e.File.Hash == f.Hash { // ничего нового
			return e.Path, false, nil
		}
	}
	for _, e := range list {
		if e.File.Origin == OriginAPI && f.Origin == OriginAPI && e.File.AccountID == f.AccountID && e.File.Kind == f.Kind {
			_ = os.Remove(e.Path)
		}
	}
	return s.Put(f)
}

// Find — файл по отпечатку.
func (s Store) Find(hash string) (Entry, bool) {
	list, _ := s.List()
	for _, e := range list {
		if e.File.Hash == hash {
			return e, true
		}
	}
	return Entry{}, false
}

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"economic/pkg/db"
	"economic/pkg/source"
)

// ───────────── Проверка файла перед добавлением/обновлением ─────────────
//
// Перед записью файл разбирается и сравнивается с тем, что уже есть (кеш + файлы-источники):
//
//	«Добавить»  — только НОВЫЙ счёт. Если счёт уже есть, его можно обновить этим файлом (явным выбором).
//	«Обновить»  — только ЭТОТ счёт. Файл другого счёта отклоняется.
//
// Алиас = имя файла. У существующего счёта алиас уже есть (к нему могут быть привязаны лица), поэтому при расхождении имён
// пользователь решает: оставить прежний или принять новый.

type problem struct {
	Code     string `json:"code"` // other_account | exists | duplicate | alias | no_match
	Text     string `json:"text"`
	Blocking bool   `json:"blocking"` // без решения пользователя записывать нельзя
	Choice   string `json:"choice,omitempty"`
}

type inspection struct {
	Kind      string `json:"kind"`
	AccountID string `json:"account_id"`
	Label     string `json:"label"` // IBAN / «Binance» / договор
	Alias     string `json:"alias"` // имя файла
	Count     int    `json:"count"`
	From      int64  `json:"from"`
	To        int64  `json:"to"`

	Exists        bool   `json:"exists"`
	ExistingAlias string `json:"existing_alias,omitempty"`
	ExistingFrom  int64  `json:"existing_from,omitempty"`
	ExistingTo    int64  `json:"existing_to,omitempty"`
	ExistingCount int    `json:"existing_count,omitempty"`

	Overlap  *overlapInfo `json:"overlap,omitempty"`
	Problems []problem    `json:"problems"`
}

// overlapInfo — как файл ложится на уже имеющиеся операции в общем отрезке.
type overlapInfo struct {
	From    int64 `json:"from"`
	To      int64 `json:"to"`
	Known   int   `json:"known"`   // есть и там, и там
	New     int   `json:"new"`     // только в файле
	Missing int   `json:"missing"` // только в уже имеющихся
}

// aliasOf — алиас существующего счёта: имя без хвоста « (тип)».
func aliasOf(a db.Account) string {
	return strings.TrimSuffix(a.Name, " ("+a.Type+")")
}

func (s *Server) findAccount(id string) (db.Account, bool) {
	accs, _ := s.storage.GetAccounts()
	for _, a := range accs {
		if a.ID == id {
			return a, true
		}
	}
	return db.Account{}, false
}

// inspect сравнивает разобранный файл с имеющимся. target — счёт, который «обновляют» ("" — режим «добавить»).
func (s *Server) inspect(f *source.File, target string) *inspection {
	in := &inspection{Kind: f.Kind, AccountID: f.AccountID, Alias: f.Name, Count: f.Count(), From: f.From, To: f.To, Problems: []problem{}}
	if cf, ct := f.Cover(); ct > 0 {
		in.From, in.To = cf, ct
	}
	in.Label = f.Card
	if f.Account != nil && f.Account.IBAN != "" {
		in.Label = f.Account.IBAN
	}

	entries, _ := store().List()
	var have []source.Entry
	for _, e := range entries {
		if e.File.AccountID == f.AccountID {
			have = append(have, e)
		}
		if e.File.Hash == f.Hash {
			in.Problems = append(in.Problems, problem{Code: "duplicate", Blocking: true, Text: "Такой источник уже есть: те же данные уже загружены (имя файла не важно)."})
		}
	}
	for _, e := range have {
		in.ExistingCount += e.File.Count()
		cf, ct := e.File.Cover()
		if in.ExistingFrom == 0 || (cf > 0 && cf < in.ExistingFrom) {
			in.ExistingFrom = cf
		}
		if ct > in.ExistingTo {
			in.ExistingTo = ct
		}
	}
	acc, inCache := s.findAccount(f.AccountID)
	in.Exists = inCache || len(have) > 0
	if inCache {
		in.ExistingAlias = aliasOf(acc)
	} else if len(have) > 0 {
		in.ExistingAlias = have[0].File.Name
	}

	// другой счёт при «Обновить»
	if target != "" && target != f.AccountID {
		name := target
		if a, ok := s.findAccount(target); ok {
			name = a.Name
		}
		in.Problems = append(in.Problems, problem{Code: "other_account", Blocking: true,
			Text: fmt.Sprintf("Файл относится к другому счёту (%s), а не к «%s». Чтобы добавить новый счёт, используй «Добавить».", in.Label, name)})
		return in
	}
	// «Добавить» нашло счёт, который уже есть
	if target == "" && in.Exists {
		in.Problems = append(in.Problems, problem{Code: "exists", Blocking: true, Choice: "update",
			Text: fmt.Sprintf("Счёт уже есть («%s»). «Добавить» создаёт только новые счета — этим файлом можно обновить существующий.", in.ExistingAlias)})
	}
	// алиас: у существующего счёта он уже есть
	if in.Exists && in.ExistingAlias != "" && strings.TrimSpace(in.ExistingAlias) != strings.TrimSpace(f.Name) {
		in.Problems = append(in.Problems, problem{Code: "alias", Choice: "alias",
			Text: fmt.Sprintf("У счёта уже есть алиас «%s», а файл называется «%s». К алиасу могут быть привязаны лица — выбери, какое имя оставить.", in.ExistingAlias, f.Name)})
	}
	// совпадение с имеющимися операциями в общем отрезке
	if in.Exists && f.Kind == source.KindBank && len(f.Txs) > 0 {
		if ov := s.overlapOf(f, have); ov != nil {
			in.Overlap = ov
			if ov.Known == 0 && ov.Missing > 0 {
				in.Problems = append(in.Problems, problem{Code: "no_match", Choice: "force",
					Text: fmt.Sprintf("В общем периоде у файла и у уже имеющихся данных нет ни одной общей операции (в файле другие: %d, в кэше: %d). Возможно, это не та выписка.", ov.New, ov.Missing)})
			}
		}
	}
	return in
}

// overlapOf сравнивает id операций файла и уже имеющихся (кеш + файлы-источники) внутри пересечения периодов.
func (s *Server) overlapOf(f *source.File, have []source.Entry) *overlapInfo {
	exist := map[string]int64{} // id → время
	cf, ct := f.From, f.To
	if ids, err := s.storage.AccountTxIDs(f.AccountID, cf, ct); err == nil {
		for id := range ids {
			exist[id] = 0
		}
	}
	for _, e := range have {
		for _, t := range e.File.Txs {
			if t.Time >= cf && t.Time <= ct {
				exist[t.ID] = t.Time
			}
		}
	}
	if len(exist) == 0 {
		return nil
	}
	ov := &overlapInfo{From: cf, To: ct}
	mine := map[string]bool{}
	for _, t := range f.Txs {
		mine[t.ID] = true
		if _, ok := exist[t.ID]; ok {
			ov.Known++
		} else {
			ov.New++
		}
	}
	for id := range exist {
		if !mine[id] {
			ov.Missing++
		}
	}
	return ov
}

// HandleInspectSource — POST /api/sources/inspect (multipart: file, account?) → что это за файл и что с ним не так. Ничего не пишет.
func (s *Server) HandleInspectSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	name := "statement"
	if header != nil && header.Filename != "" {
		name = header.Filename
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file bytes", http.StatusBadRequest)
		return
	}
	f, err := parseSource(name, raw)
	if err != nil {
		code := http.StatusInternalServerError
		if he, ok := err.(*importError); ok {
			code = he.code
		}
		http.Error(w, err.Error(), code)
		return
	}
	f.Seal()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.inspect(f, r.FormValue("account")))
}

// enforceImportRules применяет решения пользователя из формы импорта. Без поля mode — прежнее поведение (просто импорт).
//
//	mode=add     новый счёт; если счёт уже есть, нужно подтверждение update=1
//	mode=update  account=<id>: файл обязан относиться к этому счёту
//	alias=keep   оставить алиас существующего счёта (иначе принимается имя файла)
//	force=1      записать, даже если общие операции не совпали
func (s *Server) enforceImportRules(f *source.File, r *http.Request) error {
	mode := r.FormValue("mode")
	if mode == "" {
		return nil
	}
	f.Seal()
	target := ""
	if mode == "update" {
		target = r.FormValue("account")
	}
	in := s.inspect(f, target)
	for _, p := range in.Problems {
		switch p.Code {
		case "duplicate", "other_account":
			return &importError{http.StatusConflict, p.Text}
		case "exists":
			if r.FormValue("update") != "1" {
				return &importError{http.StatusConflict, p.Text}
			}
		case "no_match":
			if r.FormValue("force") != "1" {
				return &importError{http.StatusConflict, p.Text}
			}
		}
	}
	if in.Exists && r.FormValue("alias") == "keep" && in.ExistingAlias != "" {
		f.Name = in.ExistingAlias
	}
	return nil
}

package api

import (
	"io"
	"log"
	"net/http"

	"economic/pkg/source"
)

func (s *Server) HandleImportCSV(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32<<20) // выписка больше 32 МБ — это не выписка
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "Failed to read uploaded file")
		return
	}
	defer file.Close()

	filename := "statement"
	if header != nil && header.Filename != "" {
		filename = header.Filename
	}

	rawBytes, err := io.ReadAll(file)
	if err != nil {
		fail(w, http.StatusBadRequest, "Failed to read file bytes")
		return
	}

	files, archived, err := parseUpload(filename, rawBytes)
	// Проверки правил — для всей партии до первой записи: отказ одного файла не оставляет архив загруженным наполовину.
	skipped := 0
	var todo []*source.File
	if err == nil {
		for _, f := range files {
			if archived { // повторно загруженный архив: уже имеющиеся файлы пропускаем, новые добавляем
				f.Seal()
				if hasProblem(s.inspect(f, r.FormValue("account")), "duplicate") {
					skipped++
					continue
				}
			}
			if err = s.enforceImportRules(f, r); err != nil {
				break
			}
			todo = append(todo, f)
		}
	}
	if err == nil && len(todo) == 0 {
		err = &importError{http.StatusConflict, "Все файлы архива уже загружены: те же данные уже есть (имя файла не важно)."}
	}
	var res *importResult
	cachedAll := true
	if err == nil {
		res = &importResult{}
		for _, f := range todo {
			one, aerr := s.applySource(f)
			if aerr != nil {
				err = aerr
				break
			}
			res.AccountID, res.IBAN, res.Card = one.AccountID, one.IBAN, one.Card
			res.Count += one.Count
			// Разобрано → кладём файл-источник (.zoe) на диск, если таких данных там ещё нет (по отпечатку содержимого).
			// Это НЕ кэш-база: переживает любой сброс, из него «Загрузить» восстанавливает данные.
			_, cached, perr := store().Put(f)
			if perr != nil {
				log.Printf("[Sources] не удалось сохранить источник %q: %v", f.Name, perr)
			}
			cachedAll = cachedAll && cached
		}
	}
	if err != nil {
		code := http.StatusInternalServerError
		if he, ok := err.(*importError); ok {
			code = he.code
		}
		fail(w, code, err.Error())
		return
	}

	out := map[string]interface{}{
		"status":         "success",
		"account_id":     res.AccountID,
		"iban":           res.IBAN,
		"card_number":    res.Card,
		"imported_count": res.Count,
		"source_cached":  cachedAll,
	}
	if archived {
		out["files"] = len(todo)
		out["skipped"] = skipped
	}
	writeJSON(w, out)
}

// hasProblem — в проверке файла есть проблема с таким кодом.
func hasProblem(in *inspection, code string) bool {
	for _, p := range in.Problems {
		if p.Code == code {
			return true
		}
	}
	return false
}

type importError struct {
	code int
	msg  string
}

func (e *importError) Error() string { return e.msg }

type importResult struct {
	AccountID, IBAN, Card string
	Count                 int
}

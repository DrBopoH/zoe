package api

import (
	"io"
	"log"
	"net/http"
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

	f, err := parseSource(filename, rawBytes)
	if err == nil {
		err = s.enforceImportRules(f, r)
	}
	var res *importResult
	if err == nil {
		res, err = s.applySource(f)
	}
	if err != nil {
		code := http.StatusInternalServerError
		if he, ok := err.(*importError); ok {
			code = he.code
		}
		fail(w, code, err.Error())
		return
	}

	// Разобрано → кладём файл-источник (.zoe) на диск, если таких данных там ещё нет (по отпечатку содержимого).
	// Это НЕ кэш-база: переживает любой сброс, из него «Загрузить» восстанавливает данные.
	_, cached, aerr := store().Put(f)
	if aerr != nil {
		log.Printf("[Sources] не удалось сохранить источник %q: %v", filename, aerr)
	}

	writeJSON(w, map[string]interface{}{
		"status":         "success",
		"account_id":     res.AccountID,
		"iban":           res.IBAN,
		"card_number":    res.Card,
		"imported_count": res.Count,
		"source_cached":  cached,
	})
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

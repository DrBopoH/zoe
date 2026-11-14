package api

import (
	"compress/gzip"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// Общий слой HTTP-ответов: вместо 77 ручных http.Error и 58 json.NewEncoder в каждом хендлере.
// Тексты ошибок и форма ответов прежние — фронт на них опирается.

const maxJSONBody = 1 << 20 // JSON-тела хендлеров маленькие; большие (выписки) читают свои хендлеры

// writeJSON — 200 + JSON. Ошибку кодирования писать некуда (заголовок уже ушёл), поэтому только лог.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[HTTP] ответ не записан: %v", err)
	}
}

// writeOK — {"status":"success"}, ответ всех «записывающих» хендлеров.
func writeOK(w http.ResponseWriter) { writeJSON(w, map[string]string{"status": "success"}) }

// writeMsg — {"status":…, "message":…}.
func writeMsg(w http.ResponseWriter, status, message string) {
	writeJSON(w, map[string]string{"status": status, "message": message})
}

func fail(w http.ResponseWriter, code int, msg string) { http.Error(w, msg, code) }

// failErr — внутренняя ошибка (500); текст остаётся в ответе: приложение однопользовательское, локальное.
func failErr(w http.ResponseWriter, err error) {
	log.Printf("[HTTP] %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// readJSON читает тело в v (не больше maxJSONBody); при ошибке сам отвечает 400 и возвращает false.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "Invalid payload")
		return false
	}
	return true
}

// allow пропускает только перечисленные методы; иначе отвечает 405 и возвращает false.
func allow(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, m := range methods {
		if r.Method == m {
			return true
		}
	}
	fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	return false
}

// writeMethods — методы записи (раньше везде «POST или PUT»).
var writeMethods = []string{http.MethodPost, http.MethodPut}

// gzipWriter сжимает только JSON-ответы: тип известен к первой записи (writeJSON ставит заголовок заранее),
// текстовые ошибки http.Error уходят как были.
type gzipWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	decided bool
}

func (g *gzipWriter) decide() {
	if g.decided {
		return
	}
	g.decided = true
	h := g.Header()
	if strings.HasPrefix(h.Get("Content-Type"), "application/json") && h.Get("Content-Encoding") == "" {
		h.Set("Content-Encoding", "gzip")
		h.Del("Content-Length")
		g.gz = gzip.NewWriter(g.ResponseWriter)
	}
	h.Add("Vary", "Accept-Encoding")
}

func (g *gzipWriter) WriteHeader(code int) {
	g.decide()
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	g.decide()
	if g.gz != nil {
		return g.gz.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

func (g *gzipWriter) close() {
	if g.gz != nil {
		if err := g.gz.Close(); err != nil {
			log.Printf("[HTTP] gzip не закрыт: %v", err)
		}
	}
}

// gzipJSON сжимает JSON в ответах на GET, если клиент умеет gzip: список транзакций на десятки тысяч строк
// сжимается в разы, а на localhost и по домашней сети (телефон) это заметно по скорости загрузки.
func gzipJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next(w, r)
			return
		}
		g := &gzipWriter{ResponseWriter: w}
		defer g.close()
		next(g, r)
	}
}

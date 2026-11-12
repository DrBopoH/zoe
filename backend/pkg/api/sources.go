package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ───────────── Кэш источников ─────────────
//
// Это НЕ кэш-база (cache.db). Это папка на диске с самими файлами выписок:
//
//	~/.local/share/economic/sources/<PAN с маской>/<хэш>_<исходное имя>
//
// • новизна файла определяется по хэшу СОДЕРЖИМОГО (sha256), а не по имени;
// • папка = номер карты (не IBAN: один IBAN может нести два PAN), посередине звёздочки: 516875******8629;
// • сброс кэша-базы источники не трогает, кнопка «Загрузить» заново разбирает их все в базу.
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

func safeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r == 0 || r == '/' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		name = "statement"
	}
	if len(name) > 150 {
		ext := filepath.Ext(name)
		if len(ext) > 10 {
			ext = ""
		}
		name = name[:150-len(ext)] + ext
	}
	return name
}

func contentHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:20]
}

// archiveSource кладёт файл в кэш источников, если его содержимого там ещё нет (в ЛЮБОЙ папке).
// Возвращает true, если файл был новым и записан.
func archiveSource(card, filename string, raw []byte) (bool, error) {
	root := sourcesDir()
	hash := contentHash(raw)
	if m, _ := filepath.Glob(filepath.Join(root, "*", hash+"_*")); len(m) > 0 {
		return false, nil
	}
	dir := filepath.Join(root, maskPAN(card))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}
	dst := filepath.Join(dir, hash+"_"+safeFileName(filename))
	tmp := dst + ".tmp"
	// файлы содержат персональные данные (ФИО, паспорт, адрес) — доступ только владельцу
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return false, err
	}
	return true, nil
}

type cachedSource struct {
	Path, Name string // Name — исходное имя файла (без хэша)
}

func listSources() ([]cachedSource, error) {
	root := sourcesDir()
	var out []cachedSource
	dirs, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, d.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || strings.HasSuffix(f.Name(), ".tmp") {
				continue
			}
			name := f.Name()
			if i := strings.Index(name, "_"); i == 20 {
				name = name[i+1:]
			}
			out = append(out, cachedSource{Path: filepath.Join(root, d.Name(), f.Name()), Name: name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

type loadFailure struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

// HandleLoadSources — кнопка «Загрузить»: каждый файл из кэша источников разбирается и пишется в базу.
func (s *Server) HandleLoadSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	files, err := listSources()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	loaded, txCount := 0, 0
	failed := []loadFailure{}
	for _, f := range files {
		raw, err := os.ReadFile(f.Path)
		if err != nil {
			failed = append(failed, loadFailure{f.Name, err.Error()})
			continue
		}
		res, err := s.importSource(f.Name, raw)
		if err != nil {
			log.Printf("[Sources] %s: %v", f.Path, err)
			failed = append(failed, loadFailure{f.Name, err.Error()})
			continue
		}
		loaded++
		txCount += res.Count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "success",
		"total":        len(files),
		"loaded":       loaded,
		"transactions": txCount,
		"failed":       failed,
		"dir":          sourcesDir(),
	})
}

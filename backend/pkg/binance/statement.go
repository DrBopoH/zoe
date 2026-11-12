package binance

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Официальная выписка Binance (Wallet → Transaction History → Export) — единственный источник движений.
// В ней есть всё: спот-сделки, Earn, Strategy, фьючерсы, конвертации, P2P, депозиты. Читаются ВСЕ строки как есть;
// что с ними делать (в какой кошелёк, внешнее движение или внутреннее) решает ledger.go.

// Row — строка выписки.
type Row struct {
	T       int64 // мс
	Account string
	Op      string
	Asset   string
	Delta   float64
	Note    string
}

// Statement — разобранная выписка.
type Statement struct {
	UserID string // «Ідентифікатор користувача»: единственное, что в выписке говорит, чей это счёт
	Rows   []Row
}

var statementHeads = []string{"ідентифікатор користувача", "user_id", "user id", "идентификатор пользователя"}

func headOf(raw []byte) string {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	if i := bytes.IndexAny(raw, ",;\r\n"); i >= 0 {
		raw = raw[:i]
	}
	return strings.ToLower(strings.TrimSpace(string(raw)))
}

// IsStatement — файл похож на выписку Binance (первая колонка — идентификатор пользователя).
func IsStatement(raw []byte) bool {
	h := headOf(raw)
	for _, s := range statementHeads {
		if h == s {
			return true
		}
	}
	return false
}

var zoneRe = regexp.MustCompile(`\(UTC([+-])(\d{1,2})(?::?(\d{2}))?\)`)

// StatementZone — часовой пояс времени в выписке: Binance пишет его в имени файла «…(UTC+2)…», без пометки — UTC.
// Имя файла пользователь может переименовать: тогда пояс не определить и время читается как UTC (его видно в «Загрузить»).
func StatementZone(filename string) *time.Location {
	m := zoneRe.FindStringSubmatch(filename)
	if m == nil {
		return time.UTC
	}
	h, _ := strconv.Atoi(m[2])
	min := 0
	if m[3] != "" {
		min, _ = strconv.Atoi(m[3])
	}
	off := h*3600 + min*60
	if m[1] == "-" {
		off = -off
	}
	return time.FixedZone("UTC"+m[1]+m[2], off)
}

// ProfileID — идентификатор счёта в core.db: «BN» + id пользователя. У Binance нет ни IBAN, ни карты;
// префикс делает его IBAN-подобным (две буквы и цифры), поэтому он проходит тем же путём, что IBAN у банков.
func ProfileID(userID string) string { return "BN" + strings.TrimSpace(userID) }

// ParseStatement разбирает выписку целиком. Строки с нулевым изменением пропускаются.
func ParseStatement(r io.Reader, loc *time.Location) (*Statement, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if !IsStatement(raw) {
		return nil, errors.New("не похоже на выписку Binance (ожидается «Ідентифікатор користувача, Час, Акаунт, Операція, Монета, Змінити»)")
	}
	cr := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if loc == nil {
		loc = time.UTC
	}
	st := &Statement{}
	for _, row := range rows[1:] {
		if len(row) < 6 {
			continue
		}
		uid := strings.TrimSpace(row[0])
		if st.UserID == "" {
			st.UserID = uid
		} else if uid != st.UserID {
			return nil, errors.New("в выписке несколько пользователей: " + st.UserID + " и " + uid)
		}
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(row[1]), loc)
		if err != nil {
			return nil, errors.New("не разобрать время «" + row[1] + "»")
		}
		d, err := strconv.ParseFloat(strings.TrimSpace(row[5]), 64)
		if err != nil {
			return nil, errors.New("не разобрать сумму «" + row[5] + "»")
		}
		if d == 0 {
			continue
		}
		note := ""
		if len(row) > 6 {
			note = strings.TrimSpace(row[6])
		}
		st.Rows = append(st.Rows, Row{
			T: t.UnixMilli(), Account: strings.TrimSpace(row[2]), Op: strings.TrimSpace(row[3]),
			Asset: strings.TrimSpace(row[4]), Delta: d, Note: note,
		})
	}
	if st.UserID == "" {
		return nil, errors.New("в выписке нет строк")
	}
	return st, nil
}

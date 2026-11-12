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

// Официальная выписка Binance (Wallet → Transaction History → Export) — единственный источник движений, которых нет в API:
// переводы в Strategy (грид-боты) и на фьючерсы, стейкинг SOL, конвертация «пыли» в BNB, кэшбэк и возврат комиссий.
// Фьючерсы, пыль и стейкинг API тоже отдаёт (последние 6 месяцев и т.п.), но глубже или при сбое их видит только выписка:
// их движения помечены «stmt:» и в историю попадают лишь там, где API того же события не показал (mergeStatement).
// Всё остальное (депозиты, сделки, конвертации, P2P, Earn) приходит из API, поэтому из выписки берутся только эти операции:
// иначе движения задвоились бы (WBETH-стейкинг тоже уже виден в API: сверено по необъяснённому остатку). Строки самих разделов Strategy и Futures не нужны — внутри них капитал не меняется.

// statementOps — операция в строках счёта Spot → вид движения.
var statementOps = map[string]string{
	"Transfer Between Spot and Strategy":   "strategy",
	"Transfer Between Spot and UM Futures": "stmt:futures", // «stmt:» — то же видит и API: берём, только если API это не отдал (mergeStatement)
	"SOL Staking - Purchase":               "stmt:staking",
	"Small Assets Exchange BNB":            "stmt:dust",
	"Cashback Voucher":                     "bonus",
	"Strategy Trading Fee Rebate":          "bonus",
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

// ParseStatement разбирает выписку и отдаёт движения счёта Spot из statementOps. total — сколько строк было в файле.
func ParseStatement(r io.Reader, loc *time.Location) (flows []Flow, total int, err error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, err
	}
	if !IsStatement(raw) {
		return nil, 0, errors.New("не похоже на выписку Binance (ожидается «Ідентифікатор користувача, Час, Акаунт, Операція, Монета, Змінити»)")
	}
	cr := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, 0, err
	}
	if loc == nil {
		loc = time.UTC
	}
	for _, row := range rows[1:] {
		if len(row) < 6 {
			continue
		}
		total++
		kind, ok := statementOps[strings.TrimSpace(row[3])]
		if !ok || !strings.EqualFold(strings.TrimSpace(row[2]), "Spot") {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(row[1]), loc)
		if err != nil {
			return nil, 0, errors.New("не разобрать время «" + row[1] + "»")
		}
		d, err := strconv.ParseFloat(strings.TrimSpace(row[5]), 64)
		if err != nil || d == 0 {
			continue
		}
		flows = append(flows, Flow{T: t.UnixMilli(), Asset: strings.TrimSpace(row[4]), Delta: d, Kind: kind})
	}
	return flows, total, nil
}

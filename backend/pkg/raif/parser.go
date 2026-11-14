package raif

import (
	"bytes"
	"economic/pkg/stmtkit"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"economic/pkg/mono"
)

var (
	reIBANFind = regexp.MustCompile(`UA\d{27}`)
	// только в пределах одной строки и на границе слова: иначе «Рахунок: UA58300335…» давал «карту» из цифр IBAN
	reCardFind   = regexp.MustCompile(`(?i)(?:картк|карт|card)[^\d\n]*\b(\d{4}\*+\d{4}|\d{16})\b`)
	reEndBalance = regexp.MustCompile(`(?i)Баланс на кінець періоду:\s*([-−]?[\d \x{a0}]+(?:[.,]\d+)?)`)
	reDateOnly   = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}`)
)

type ParsedCSVResult struct {
	IBAN         string
	CardNumber   string
	AccountName  string
	BankType     string
	Balance      float64
	Transactions []mono.StatementItemDTO
}

func decodeWindows1251(b []byte) string {
	var buf bytes.Buffer
	for _, char := range b {
		if char < 0x80 {
			buf.WriteByte(char)
		} else {
			buf.WriteRune(cp1251ToRune(char))
		}
	}
	return buf.String()
}

func cp1251ToRune(b byte) rune {
	switch {
	case b >= 0xC0: // 0xC0…0xFF
		return rune(0x0410 + int(b-0xC0))
	case b == 0xA8:
		return 'Ё'
	case b == 0xB8:
		return 'ё'
	case b == 0xAA:
		return 'Є'
	case b == 0xBA:
		return 'є'
	case b == 0xAF:
		return 'Ї'
	case b == 0xBF:
		return 'ї'
	case b == 0xB2:
		return 'І'
	case b == 0xB3:
		return 'і'
	case b == 0xA5:
		return 'Ґ'
	case b == 0xB4:
		return 'ґ'
	case b == 0xB9:
		return '№'
	case b == 0xAB:
		return '«'
	case b == 0xBB:
		return '»'
	case b == 0x96:
		return '–'
	case b == 0x97:
		return '—'
	case b == 0xA0:
		return '\u00a0'
	default:
		return rune(b)
	}
}

// columns — номера колонок таблицы выписки; -1 — колонки нет.
type columns struct{ date, desc, mcc, amount, fee, card int }

// defaultColumns — раскладка по умолчанию, если в заголовке колонка названа иначе.
func defaultColumns() columns { return columns{date: 0, desc: 3, mcc: 4, amount: 5, fee: -1, card: -1} }

// decodeContent превращает байты файла в текст: UTF-8 как есть, иначе Windows-1251 (признак — нет слов «дата» и «деталі»).
func decodeContent(raw []byte) string {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	content := string(raw)
	low := strings.ToLower(content)
	if !strings.Contains(low, "дата") && !strings.Contains(low, "деталі") {
		return decodeWindows1251(raw)
	}
	return content
}

// statementHead — что видно в шапке документа до таблицы: IBAN, карта, баланс на конец периода.
func statementHead(content string) (iban, card string, balance float64) {
	iban = reIBANFind.FindString(content)
	if m := reCardFind.FindStringSubmatch(content); len(m) > 1 {
		card = m[1]
	}
	if m := reEndBalance.FindStringSubmatch(content); len(m) > 1 {
		balance = parseRaifAmount(m[1])
	}
	return iban, card, balance
}

// detectDelimiter определяет разделитель по строке заголовка таблицы (шапка файла может содержать запятые и «;» как угодно).
func detectDelimiter(content string) (rune, error) {
	firstLine, headerLine := "", ""
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if firstLine == "" {
			firstLine = l
		}
		low := strings.ToLower(l)
		if strings.Contains(low, "деталі операції") || strings.Contains(low, "дата і час") {
			headerLine = l
			break
		}
	}
	if firstLine == "" {
		return 0, fmt.Errorf("empty csv file")
	}
	if headerLine == "" {
		headerLine = firstLine
	}
	if strings.Count(headerLine, ",") > strings.Count(headerLine, ";") {
		return ',', nil
	}
	return ';', nil
}

// findHeaderRow — номер строки заголовка таблицы или -1.
func findHeaderRow(records [][]string) int {
	for i, row := range records {
		if len(row) < 4 {
			continue
		}
		joined := strings.ToLower(strings.Join(row, " "))
		if strings.Contains(joined, "деталі операції") || strings.Contains(joined, "номер картки") || (strings.Contains(joined, "дата і час") && strings.Contains(joined, "сума")) {
			return i
		}
	}
	return -1
}

// mapColumns сопоставляет колонки по названиям в заголовке; не названные остаются по умолчанию.
func mapColumns(header []string) columns {
	col := defaultColumns()
	for j, cell := range header {
		c := strings.ToLower(strings.TrimSpace(cell))
		switch {
		case c == "дата і час здійснення операції" || c == "дата":
			col.date = j
		case c == "деталі операції" || c == "опис":
			col.desc = j
		case c == "mcc":
			col.mcc = j
		case c == "сума у валюті рахунку":
			col.amount = j
		case strings.Contains(c, "комісі"):
			col.fee = j
		case strings.Contains(c, "картк") || strings.Contains(c, "pan"):
			col.card = j
		}
	}
	return col
}

// counterOf — имя контрагента из описания «Тип: Имя» (если после двоеточия не просто дата).
func counterOf(desc string) string {
	if !strings.Contains(desc, ":") {
		return ""
	}
	val := strings.TrimSpace(strings.SplitN(desc, ":", 2)[1])
	if reDateOnly.MatchString(val) {
		return ""
	}
	return val
}

// parseRow разбирает строку таблицы; ok=false — строка не операция (пустая, без даты или суммы). card — карта из колонки, если есть.
func parseRow(row []string, col columns, iban string, seen map[string]int) (item mono.StatementItemDTO, card string, ok bool) {
	if len(row) <= col.date || len(row) <= col.amount {
		return item, "", false
	}
	dateStr, amountStr := strings.TrimSpace(row[col.date]), strings.TrimSpace(row[col.amount])
	if dateStr == "" || amountStr == "" {
		return item, "", false
	}
	if col.card != -1 && len(row) > col.card {
		card = strings.TrimSpace(row[col.card])
	}
	txTime := parseRaifDate(dateStr)
	if txTime.IsZero() {
		return item, card, false
	}

	amount, fee := parseRaifAmount(amountStr), 0.0
	if col.fee != -1 && len(row) > col.fee {
		fee = parseRaifAmount(row[col.fee])
	}
	// Комиссия входит в реальное изменение баланса и при расходе, и при приходе: -5410.00 − 59.10 = -5469.10
	amountCents, feeCents, opCents := stmtkit.Cents(amount-fee), stmtkit.Cents(fee), stmtkit.Cents(amount)

	desc := ""
	if len(row) > col.desc {
		desc = strings.TrimSpace(row[col.desc])
	}
	mcc := 0
	if len(row) > col.mcc {
		mcc, _ = strconv.Atoi(strings.TrimSpace(row[col.mcc]))
	}
	return mono.StatementItemDTO{
		ID:              txID(iban, txTime.Unix(), opCents, feeCents, desc, seen),
		Time:            txTime.Unix(),
		Description:     desc,
		CounterName:     counterOf(desc),
		MCC:             mcc,
		Amount:          amountCents,
		OperationAmount: opCents,
		CommissionRate:  feeCents,
		CurrencyCode:    980,
	}, card, true
}

// ParseCSV разбирает выписку Райффайзена (UTF-8 или Windows-1251, разделитель «;» или «,»).
func ParseCSV(r io.Reader) (*ParsedCSVResult, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	content := decodeContent(raw)
	iban, card, balance := statementHead(content)

	delimiter, err := detectDelimiter(content)
	if err != nil {
		return nil, err
	}
	csvReader := csv.NewReader(strings.NewReader(content))
	csvReader.Comma = delimiter
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	headerIdx := findHeaderRow(records)
	if headerIdx == -1 {
		return nil, fmt.Errorf("table header not found in CSV")
	}
	col := mapColumns(records[headerIdx])

	var items []mono.StatementItemDTO
	seen := make(map[string]int)
	for _, row := range records[headerIdx+1:] {
		item, rowCard, ok := parseRow(row, col, iban, seen)
		if rowCard != "" && card == "" {
			card = rowCard // карта из шапки важнее, колонка — запасной источник
		}
		if ok {
			items = append(items, item)
		}
	}

	return &ParsedCSVResult{
		IBAN:         iban,
		CardNumber:   card,
		AccountName:  "Raiffeisen Bank",
		BankType:     "raif",
		Balance:      balance,
		Transactions: items,
	}, nil
}

func parseRaifDate(s string) time.Time {
	layouts := []string{
		"02.01.2006 15:04:05",
		"02.01.2006 15:04",
		"02.01.2006",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	clean := strings.TrimSpace(s)
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, clean, stmtkit.Kyiv); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseRaifAmount(s string) float64 {
	clean := strings.NewReplacer(" ", "", "\u00a0", "", "\u202f", "", "−", "-", ",", ".").Replace(strings.TrimSpace(s))
	val, _ := strconv.ParseFloat(clean, 64)
	return val
}

// toCents — float → целые копейки без потери (int64(54.10*100) давал 5409).
// txID — ID операции для выписки Райффайзена (см. stmtkit.StableID).
func txID(account string, ts, op, fee int64, desc string, seen map[string]int) string {
	return stmtkit.StableID("csv_raif_", fmt.Sprintf("%s|%d|%d|%d|%s", account, ts, op, fee, desc), seen)
}

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

func ParseCSV(r io.Reader) (*ParsedCSVResult, error) {
	rawBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	rawBytes = bytes.TrimPrefix(rawBytes, []byte("\xef\xbb\xbf"))

	content := string(rawBytes)
	if !strings.Contains(strings.ToLower(content), "дата") && !strings.Contains(strings.ToLower(content), "деталі") {
		content = decodeWindows1251(rawBytes)
	}

	// 1. Извлечение IBAN, карты и Баланса из шапки документа
	detectedIBAN := reIBANFind.FindString(content)

	detectedCard := ""
	if match := reCardFind.FindStringSubmatch(content); len(match) > 1 {
		detectedCard = match[1]
	}

	endBalance := 0.0
	if match := reEndBalance.FindStringSubmatch(content); len(match) > 1 {
		endBalance = parseRaifAmount(match[1])
	}

	// разделитель определяем по строке заголовка таблицы (шапка файла может содержать запятые и «;» как угодно)
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
		return nil, fmt.Errorf("empty csv file")
	}
	if headerLine == "" {
		headerLine = firstLine
	}
	delimiter := ';'
	if strings.Count(headerLine, ",") > strings.Count(headerLine, ";") {
		delimiter = ','
	}

	csvReader := csv.NewReader(strings.NewReader(content))
	csvReader.Comma = delimiter
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	// 2. Поиск строки заголовка таблицы
	headerIdx := -1
	for i, row := range records {
		if len(row) < 4 {
			continue
		}
		rowJoined := strings.ToLower(strings.Join(row, " "))
		if strings.Contains(rowJoined, "деталі операції") || strings.Contains(rowJoined, "номер картки") || (strings.Contains(rowJoined, "дата і час") && strings.Contains(rowJoined, "сума")) {
			headerIdx = i
			break
		}
	}

	if headerIdx == -1 {
		return nil, fmt.Errorf("table header not found in CSV")
	}

	// 3. Сопоставление колонок таблицы
	dateCol := 0
	descCol := 3
	mccCol := 4
	amountCol := 5
	feeCol := -1
	cardCol := -1

	for j, cell := range records[headerIdx] {
		c := strings.ToLower(strings.TrimSpace(cell))
		if c == "дата і час здійснення операції" || c == "дата" {
			dateCol = j
		} else if c == "деталі операції" || c == "опис" {
			descCol = j
		} else if c == "mcc" {
			mccCol = j
		} else if c == "сума у валюті рахунку" {
			amountCol = j
		} else if strings.Contains(c, "комісі") {
			feeCol = j
		} else if strings.Contains(c, "картк") || strings.Contains(c, "pan") {
			cardCol = j
		}
	}

	var items []mono.StatementItemDTO
	seen := make(map[string]int)

	for i := headerIdx + 1; i < len(records); i++ {
		row := records[i]
		if len(row) <= dateCol || len(row) <= amountCol {
			continue
		}

		dateStr := strings.TrimSpace(row[dateCol])
		amountStr := strings.TrimSpace(row[amountCol])
		if dateStr == "" || amountStr == "" {
			continue
		}

		// Извлечение карты из строки таблицы
		if detectedCard == "" && cardCol != -1 && len(row) > cardCol {
			cVal := strings.TrimSpace(row[cardCol])
			if cVal != "" {
				detectedCard = cVal
			}
		}

		txTime := parseRaifDate(dateStr)
		if txTime.IsZero() {
			continue
		}

		amountFloat := parseRaifAmount(amountStr)
		feeFloat := 0.0
		if feeCol != -1 && len(row) > feeCol {
			feeFloat = parseRaifAmount(row[feeCol])
		}

		// Учитываем комиссию в реальном изменении баланса счета:
		// Если это расход (amountFloat < 0), то итоговое списание = amountFloat - feeFloat (-5410.00 - 59.10 = -5469.10)
		// Если это приход (amountFloat > 0), то зачисление = amountFloat - feeFloat
		realAmountFloat := amountFloat
		if amountFloat < 0 {
			realAmountFloat -= feeFloat
		} else {
			realAmountFloat -= feeFloat
		}

		amountCents := stmtkit.Cents(realAmountFloat)
		feeCents := stmtkit.Cents(feeFloat)
		opCents := stmtkit.Cents(amountFloat)

		desc := ""
		if len(row) > descCol {
			desc = strings.TrimSpace(row[descCol])
		}

		mccVal := 0
		if len(row) > mccCol {
			mccVal, _ = strconv.Atoi(strings.TrimSpace(row[mccCol]))
		}

		counterName := ""
		counterIban := ""

		if strings.Contains(desc, ":") {
			parts := strings.SplitN(desc, ":", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				if !reDateOnly.MatchString(val) {
					counterName = val
				}
			}
		}

		items = append(items, mono.StatementItemDTO{
			ID:              txID(detectedIBAN, txTime.Unix(), opCents, feeCents, desc, seen),
			Time:            txTime.Unix(),
			Description:     desc,
			CounterName:     counterName,
			CounterIban:     counterIban,
			MCC:             mccVal,
			Amount:          amountCents,
			OperationAmount: opCents,
			CommissionRate:  feeCents,
			CurrencyCode:    980,
		})
	}

	return &ParsedCSVResult{
		IBAN:         detectedIBAN,
		CardNumber:   detectedCard,
		AccountName:  "Raiffeisen Bank",
		BankType:     "raif",
		Balance:      endBalance,
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

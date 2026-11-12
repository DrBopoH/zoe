// Package novapay разбирает CSV-выписку NovaPay («Період формування … Дата та час операції;…;Сума операції;Комісія;Залишок»).
// Персональные данные шапки (ПІБ, ІПН, паспорт) не читаются и никуда не попадают — берутся только счёт, карта и остаток.
package novapay

import (
	"bytes"
	"economic/pkg/stmtkit"
	"encoding/csv"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"economic/pkg/mono"
)

const BankType = "novapay"

var (
	reIBAN    = regexp.MustCompile(`UA\d{27}`)
	reCardHdr = regexp.MustCompile(`Платіжна картка:\s*;?\s*(\d{4})[\s*]+(\d{4})`)
	reBalance = regexp.MustCompile(`Залишок на кінець періоду:\s*;?\s*([-−]?[\d \x{a0}]+(?:[.,]\d+)?)`)
	// «від Ivanenko Petro|адрес|… 414951***5262» — имя отправителя до первой «|»
	reFrom = regexp.MustCompile(`(?:^|\s)від\s+([^|,]+?)\s*\|`)
	// отправитель/получатель: карта вида 414951***5262 или 414951xxxxxx5262
	reCounterCard = regexp.MustCompile(`(\d{4})\d{2}[*xX•]{3,6}(\d{4})`)
	reRecipient   = regexp.MustCompile(`отримувач:\s*([^,]+)`)
	reMCC         = regexp.MustCompile(`,\s*(\d{4}),\s*від\s`)
	reLegalForm   = regexp.MustCompile(`(?i)^\s*товариство\s+з\s+обмеженою\s+відповідальністю\s*`)
)

type Result struct {
	IBAN         string
	CardNumber   string // 5175****0170
	Balance      float64
	Transactions []mono.StatementItemDTO
}

func text(raw []byte) string { return string(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))) }

// IsStatement — признаки выписки NovaPay: шапка с платёжной карточкой и таблица «Дата та час операції … Залишок після операції».
func IsStatement(raw []byte) bool {
	s := text(raw)
	return strings.Contains(s, "Платіжна картка") && strings.Contains(s, "Дата та час операції") && strings.Contains(s, "Залишок після операції")
}

func ParseCSV(raw []byte) (*Result, error) {
	s := text(raw)
	res := &Result{IBAN: reIBAN.FindString(s)}
	if m := reCardHdr.FindStringSubmatch(s); m != nil {
		res.CardNumber = m[1] + "****" + m[2]
	}
	if m := reBalance.FindStringSubmatch(s); m != nil {
		res.Balance = amount(m[1])
	}

	r := csv.NewReader(strings.NewReader(s))
	r.Comma, r.FieldsPerRecord, r.LazyQuotes = ';', -1, true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	head := -1
	for i, row := range records {
		if len(row) >= 6 && strings.Contains(row[0], "Дата та час операції") {
			head = i
			break
		}
	}
	if head < 0 {
		return nil, fmt.Errorf("table header not found in CSV")
	}

	seen := map[string]int{}
	for _, row := range records[head+1:] {
		if len(row) < 5 {
			continue
		}
		ts, err := time.ParseInLocation("02.01.2006 15:04", strings.TrimSpace(row[0]), stmtkit.Kyiv)
		if err != nil {
			continue
		}
		desc := strings.TrimSpace(row[3])
		cents := stmtkit.Cents(amount(row[4]))
		fee := int64(0)
		if len(row) > 5 {
			fee = stmtkit.Cents(amount(row[5])) // «-» → 0; суммы комиссий в выписке идут отдельными строками
		}
		it := mono.StatementItemDTO{
			ID: id(res.IBAN, ts.Unix(), cents, desc, seen), Time: ts.Unix(), Description: desc,
			Amount: cents, OperationAmount: cents, CommissionRate: fee, CurrencyCode: 980,
		}
		if m := reFrom.FindStringSubmatch(desc); m != nil {
			it.CounterName = strings.TrimSpace(m[1])
		} else if m := reRecipient.FindStringSubmatch(desc); m != nil {
			it.CounterName = strings.Trim(reLegalForm.ReplaceAllString(strings.TrimSpace(m[1]), ""), `"“”« »`)
		}
		it.CounterIban = CounterCard(desc) // маска карты контрагента — в то же поле, что IBAN у других банков (фронт и резолвер понимают маску)
		if m := reMCC.FindStringSubmatch(desc); m != nil {
			it.MCC, _ = strconv.Atoi(m[1])
		}
		res.Transactions = append(res.Transactions, it)
	}
	return res, nil
}

func amount(s string) float64 {
	clean := strings.NewReplacer(" ", "", " ", "", " ", "", "−", "-", ",", ".").Replace(strings.TrimSpace(s))
	v, _ := strconv.ParseFloat(clean, 64) // «-» и пусто → 0
	return v
}

// CounterCard — маска карты контрагента из описания («414951***5262» → «4149****5262»), пусто если её нет.
// Своя карта в описании зачисления («на карту № 517547******0170») идёт раньше карты отправителя — берётся последняя.
func CounterCard(desc string) string {
	all := reCounterCard.FindAllStringSubmatch(desc, -1)
	if len(all) == 0 {
		return ""
	}
	m := all[len(all)-1]
	return m[1] + "****" + m[2]
}

// id стабилен между выгрузками (см. stmtkit.StableID).
func id(account string, ts, cents int64, desc string, seen map[string]int) string {
	return stmtkit.StableID("csv_novapay_", fmt.Sprintf("%s|%d|%d|%s", account, ts, cents, desc), seen)
}

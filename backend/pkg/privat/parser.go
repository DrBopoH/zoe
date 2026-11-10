package privat

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"math"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"economic/pkg/mono"
)

// ParsedResult — результат разбора PDF-выписки ПриватБанка.
type ParsedResult struct {
	IBAN         string
	CardNumber   string
	AccountName  string
	Balance      float64
	Transactions []mono.StatementItemDTO

	HasTotals      bool
	HeaderExpenses int64 // отрицательное число
	HeaderIncome   int64
	HeaderFees     int64

	SumExpenses int64
	SumIncome   int64
	SumFees     int64
}

type ChecksumError struct {
	Expenses, HeaderExpenses int64
	Income, HeaderIncome     int64
	Fees, HeaderFees         int64
}

func (e *ChecksumError) Error() string {
	return fmt.Sprintf(
		"выписка не сошлась с шапкой: расходы %s (в шапке %s), поступления %s (в шапке %s), комиссии %s (в шапке %s)",
		fmtCents(e.Expenses), fmtCents(e.HeaderExpenses),
		fmtCents(e.Income), fmtCents(e.HeaderIncome),
		fmtCents(e.Fees), fmtCents(e.HeaderFees),
	)
}

func fmtCents(v int64) string {
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%d,%02d", sign, v/100, v%100)
}

const (
	colDateMax    = 90.0
	colCardMax    = 200.0
	colDetailsMin = 220.0
	colDetailsMax = 372.0

	edgeOp   = 435.0
	edgeCard = 490.0
	edgeFee  = 535.0

	detailsRowGap = 10.0
	lineHeightPt  = 4.0
)

var (
	reHeaderIBAN = regexp.MustCompile(`Рахунок IBAN:\s*(UA\d{27})`)
	reIBAN       = regexp.MustCompile(`UA\d{27}`)
	reCardNum    = regexp.MustCompile(`Інформація по картці:\s*([\d\*]+)`)
	reCurrency   = regexp.MustCompile(`Валюта рахунку:\s*([A-Z]{3})`)
	money        = `(-?\d{1,3}(?:[ \x{a0}\x{202f}]\d{3})*,\d{2})`
	reExpenses   = regexp.MustCompile(`Усього витрат:\s*` + money)
	reIncome     = regexp.MustCompile(`Усього надходжень:\s*` + money)
	reFees       = regexp.MustCompile(`Сума комісій:\s*` + money)
	reDateWord   = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}$`)
	reClockWord  = regexp.MustCompile(`^\d{2}:\d{2}$`)
	reAmount     = regexp.MustCompile(`^-?\d+,\d{2}$`)
	reDigits     = regexp.MustCompile(`^-?\d+$`)
	reThousands  = regexp.MustCompile(`^\d{3}(?:,\d{2})?$`)
	reSpaces     = regexp.MustCompile(`\s+`)

	reParty = regexp.MustCompile(`(?:отримувач|платник)\s+(.+?),\s+(?:банк\s+(?:отримувача|платника)|номер\s+картки|\d{4,6}[*Xx]+\d{2,4})`)
	reFrom  = regexp.MustCompile(`^від\s+(.+)$`)

	bboxTokenRe = regexp.MustCompile(`<page [^>]*>|<word xMin="([\d.]+)" yMin="([\d.]+)" xMax="([\d.]+)" yMax="([\d.]+)">([^<]*)</word>`)
)

var footerPrefixes = []string{"Вклади", "Довідка", "Керівник", "Сформовано", "Інформація"}

var currencyCodes = map[string]int{"UAH": 980, "USD": 840, "EUR": 978, "PLN": 985, "GBP": 826}

type word struct {
	x0, y0, x1, y1 float64
	text           string
}

func (w word) yc() float64 { return (w.y0 + w.y1) / 2 }

type txRow struct {
	page        int
	yc          float64
	date, clock string
	op, card    int64
	fee, disc   int64
	haveCard    bool
	haveOp      bool
	details     []string
}

func ParsePDF(r io.Reader) (*ParsedResult, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	layoutText, err := pdftotext(raw, "-layout")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(layoutText) == "" {
		return nil, fmt.Errorf("empty text extracted from PDF")
	}
	bboxText, err := pdftotext(raw, "-bbox")
	if err != nil {
		return nil, err
	}

	res := &ParsedResult{AccountName: "PrivatBank (PDF)"}

	// 1. Шапка: IBAN, номер карты, валюта, контрольные итоги
	var iban string
	if m := reHeaderIBAN.FindStringSubmatch(layoutText); len(m) > 1 {
		iban = strings.TrimSpace(m[1])
	} else {
		iban = reIBAN.FindString(layoutText)
	}

	if m := reCardNum.FindStringSubmatch(layoutText); len(m) > 1 {
		res.CardNumber = strings.TrimSpace(m[1])
	}
	res.IBAN = iban
	if res.IBAN == "" {
		res.IBAN = res.CardNumber
	}

	currency := 980
	if m := reCurrency.FindStringSubmatch(layoutText); len(m) > 1 {
		if code, ok := currencyCodes[m[1]]; ok {
			currency = code
		}
	}

	exp, okE := headerMoney(reExpenses, layoutText)
	inc, okI := headerMoney(reIncome, layoutText)
	fee, okF := headerMoney(reFees, layoutText)
	if okE && okI && okF {
		res.HasTotals = true
		res.HeaderExpenses, res.HeaderIncome, res.HeaderFees = exp, inc, fee
	}

	// 2. Строки таблицы
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		loc = time.Local
	}

	seen := make(map[string]int)
	for pageNo, words := range parseBBox(bboxText) {
		for _, row := range extractPageRows(words, pageNo+1) {
			if !row.haveCard {
				return nil, fmt.Errorf("стр. %d, операция %s %s: не найдена сумма в валюті картки", row.page, row.date, row.clock)
			}
			ts, err := time.ParseInLocation("02.01.2006 15:04", row.date+" "+row.clock, loc)
			if err != nil {
				return nil, fmt.Errorf("стр. %d: неверная дата операции %q %q: %w", row.page, row.date, row.clock, err)
			}

			desc := strings.TrimSpace(reSpaces.ReplaceAllString(strings.Join(row.details, " "), " "))
			counter := extractCounterparty(desc)

			op := row.op
			if !row.haveOp {
				op = row.card
			}

			res.Transactions = append(res.Transactions, mono.StatementItemDTO{
				ID:              txID(res.IBAN, ts.Unix(), row.card, row.fee, desc, seen),
				Time:            ts.Unix(),
				Description:     desc,
				CounterName:     counter,
				CounterIban:     extractCounterAccount(desc),
				Amount:          row.card,
				OperationAmount: op,
				CurrencyCode:    currency,
				CommissionRate:  row.fee,
				CashbackAmount:  row.disc,
			})

			switch {
			case row.card < 0:
				res.SumExpenses += row.card
			case row.card > 0:
				res.SumIncome += row.card
			}
			res.SumFees += row.fee
		}
	}

	// 3. Рассчитываем итоговый дебетовый/кредитовый баланс карты по выписке
	res.Balance = float64(res.SumIncome+res.SumExpenses) / 100.0

	// 4. Сверка с итогами из шапки
	if res.HasTotals &&
		(res.SumExpenses != res.HeaderExpenses || res.SumIncome != res.HeaderIncome || res.SumFees != res.HeaderFees) {
		return res, &ChecksumError{
			Expenses: res.SumExpenses, HeaderExpenses: res.HeaderExpenses,
			Income: res.SumIncome, HeaderIncome: res.HeaderIncome,
			Fees: res.SumFees, HeaderFees: res.HeaderFees,
		}
	}

	return res, nil
}

func pdftotext(raw []byte, mode string) (string, error) {
	cmd := exec.Command("pdftotext", mode, "-", "-")
	cmd.Stdin = bytes.NewReader(raw)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext %s failed: %w: %s", mode, err, strings.TrimSpace(stderr.String()))
	}
	return out.String(), nil
}

func headerMoney(re *regexp.Regexp, text string) (int64, bool) {
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0, false
	}
	return parseCents(m[1])
}

func parseCents(s string) (int64, bool) {
	s = strings.NewReplacer(" ", "", " ", "", " ", "").Replace(strings.TrimSpace(s))
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	i := strings.Index(s, ",")
	if i < 0 || len(s)-i-1 != 2 {
		return 0, false
	}
	whole, err1 := strconv.ParseInt(s[:i], 10, 64)
	frac, err2 := strconv.ParseInt(s[i+1:], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	v := whole*100 + frac
	if neg {
		v = -v
	}
	return v, true
}

func parseBBox(s string) [][]word {
	var pages [][]word
	for _, m := range bboxTokenRe.FindAllStringSubmatch(s, -1) {
		if strings.HasPrefix(m[0], "<page") {
			pages = append(pages, nil)
			continue
		}
		if len(pages) == 0 {
			continue
		}
		x0, _ := strconv.ParseFloat(m[1], 64)
		y0, _ := strconv.ParseFloat(m[2], 64)
		x1, _ := strconv.ParseFloat(m[3], 64)
		y1, _ := strconv.ParseFloat(m[4], 64)
		pages[len(pages)-1] = append(pages[len(pages)-1], word{x0, y0, x1, y1, html.UnescapeString(m[5])})
	}
	return pages
}

func tableTop(ws []word) float64 {
	for _, w := range ws {
		if w.x0 < colDateMax && w.text == "операції" {
			return w.y0 + 14
		}
	}
	for _, w := range ws {
		if strings.HasPrefix(w.text, "Сторінка") {
			return w.y1 + 10
		}
	}
	return 0
}

func tableBottom(ws []word, top float64) float64 {
	bottom := math.MaxFloat64
	for _, w := range ws {
		if w.y0 <= top || w.x0 >= 50 || w.y0 >= bottom {
			continue
		}
		for _, p := range footerPrefixes {
			if strings.HasPrefix(w.text, p) {
				bottom = w.y0
				break
			}
		}
	}
	return bottom
}

func mergeAmountWords(ws []word) []word {
	var rest, amt []word
	for _, w := range ws {
		if w.x1 > colDetailsMax {
			amt = append(amt, w)
		} else {
			rest = append(rest, w)
		}
	}
	sort.SliceStable(amt, func(i, j int) bool {
		if math.Abs(amt[i].y0-amt[j].y0) > 0.6 {
			return amt[i].y0 < amt[j].y0
		}
		return amt[i].x0 < amt[j].x0
	})

	merged := make([]word, 0, len(amt))
	for _, w := range amt {
		if n := len(merged); n > 0 {
			p := &merged[n-1]
			if math.Abs(p.y0-w.y0) < 0.6 && w.x0-p.x1 < 5 && reDigits.MatchString(p.text) && reThousands.MatchString(w.text) {
				p.text += w.text
				p.x1 = w.x1
				p.y1 = math.Max(p.y1, w.y1)
				continue
			}
		}
		merged = append(merged, w)
	}
	return append(rest, merged...)
}

func extractPageRows(ws []word, pageNo int) []*txRow {
	top := tableTop(ws)
	bottom := tableBottom(ws, top)

	var tw []word
	for _, w := range ws {
		if w.y0 > top && w.y0 < bottom {
			tw = append(tw, w)
		}
	}
	tw = mergeAmountWords(tw)

	var rows []*txRow
	for _, w := range tw {
		if w.x0 >= colDateMax || !reDateWord.MatchString(w.text) {
			continue
		}
		row := &txRow{page: pageNo, date: w.text, clock: "00:00", yc: w.yc()}
		for _, t := range tw {
			if t.x0 < colDateMax && reClockWord.MatchString(t.text) &&
				t.y0-w.y0 > 3 && t.y0-w.y0 < 14 && math.Abs(t.x0-w.x0) < 3 {
				row.clock = t.text
				row.yc = (w.y0 + t.y1) / 2
				break
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].yc < rows[j].yc })

	nearest := func(y float64) *txRow {
		best := rows[0]
		for _, r := range rows[1:] {
			if math.Abs(r.yc-y) < math.Abs(best.yc-y) {
				best = r
			}
		}
		return best
	}

	detailLines := make(map[int][]word)
	for _, w := range tw {
		switch {
		case w.x0 < colCardMax:

		case w.x0 >= colDetailsMin && w.x1 <= colDetailsMax:
			key := int(math.Round(w.y0 * 10))
			detailLines[key] = append(detailLines[key], w)

		case w.x1 > colDetailsMax:
			if !reAmount.MatchString(w.text) {
				continue
			}
			v, ok := parseCents(w.text)
			if !ok {
				continue
			}
			row := nearest(w.yc())
			switch {
			case w.x1 < edgeOp:
				if !row.haveOp {
					row.op, row.haveOp = v, true
				}
			case w.x1 < edgeCard:
				if !row.haveCard {
					row.card, row.haveCard = v, true
				}
			case w.x1 < edgeFee:
				row.fee = v
			default:
				row.disc = v
			}
		}
	}

	keys := make([]int, 0, len(detailLines))
	for k := range detailLines {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	type group struct {
		first, last float64
		lines       []string
	}
	var groups []*group
	for _, k := range keys {
		line := detailLines[k]
		sort.SliceStable(line, func(i, j int) bool { return line[i].x0 < line[j].x0 })
		parts := make([]string, len(line))
		for i, w := range line {
			parts[i] = w.text
		}
		y := float64(k) / 10
		if n := len(groups); n > 0 && y-groups[n-1].last <= detailsRowGap {
			groups[n-1].last = y
			groups[n-1].lines = append(groups[n-1].lines, strings.Join(parts, " "))
		} else {
			groups = append(groups, &group{first: y, last: y, lines: []string{strings.Join(parts, " ")}})
		}
	}
	for _, g := range groups {
		row := nearest((g.first + g.last + lineHeightPt) / 2)
		row.details = append(row.details, strings.Join(g.lines, " "))
	}

	return rows
}

func extractCounterparty(desc string) string {
	name := ""
	if m := reParty.FindStringSubmatch(desc); len(m) > 1 {
		name = strings.TrimSpace(m[1])
	} else if m := reFrom.FindStringSubmatch(desc); len(m) > 1 {
		name = strings.TrimSpace(m[1])
	} else if strings.HasPrefix(desc, "Виплата ") {
		name = desc
	}
	if name == "-" {
		return ""
	}
	return unifyLookalikes(name)
}

// ПриватБанк в одном и том же ФИО то пишет украинскую «І», то латинскую «I»
// (ВIКТОРIВНА / ВІКТОРІВНА) — без этого один человек распадается на двух контрагентов.
// Латинские двойники заменяются на кириллические только в словах, где уже есть кириллица.
var lookalikes = strings.NewReplacer(
	"I", "І", "i", "і", "A", "А", "a", "а", "B", "В", "C", "С", "E", "Е", "H", "Н",
	"K", "К", "M", "М", "O", "О", "P", "Р", "T", "Т", "X", "Х", "c", "с", "o", "о",
	"p", "р", "x", "х", "y", "у",
)

func unifyLookalikes(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		hasCyr, hasLat := false, false
		for _, r := range w {
			switch {
			case r >= 0x0400 && r <= 0x04FF:
				hasCyr = true
			case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
				hasLat = true
			}
		}
		if hasCyr && hasLat {
			words[i] = lookalikes.Replace(w)
		}
	}
	return strings.Join(words, " ")
}

func txID(account string, ts, card, fee int64, desc string, seen map[string]int) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d|%d|%s", account, ts, card, fee, desc)))
	base := hex.EncodeToString(sum[:])[:20]
	n := seen[base]
	seen[base] = n + 1
	if n == 0 {
		return "pdf_privat_" + base
	}
	return fmt.Sprintf("pdf_privat_%s_%d", base, n)
}

var (
	rePayerAcct = regexp.MustCompile(`номер картки/рахунку платника\s+(\S+?),\s*банк платника`)
	reRecvAcct  = regexp.MustCompile(`отримувач .+?,\s*(\d{6}\*+\d{2,4}|\d{16}),\s*банк отримувача`)
)

// extractCounterAccount — карта контрагента из описания (для резолвинга переводов, в т.ч. между своими картами):
//
//	«платник ИМЯ, номер картки/рахунку платника 545708******4946, банк платника …» → 545708******4946
//	«отримувач ИМЯ, 414949****72, банк отримувача …»                               → 414949****72
//
// У покупок (отримувач WOG 2926, банк отримувача -) счёта нет — вернётся "".
// Своя карта, стоящая ПЕРЕД словом «отримувач» (516875****29), сюда не попадает.
func extractCounterAccount(desc string) string {
	if m := rePayerAcct.FindStringSubmatch(desc); len(m) > 1 && m[1] != "-" && strings.ContainsAny(m[1], "0123456789") {
		if strings.HasPrefix(strings.ToUpper(m[1]), "UA") {
			return ""
		}
		return m[1]
	}
	if m := reRecvAcct.FindStringSubmatch(desc); len(m) > 1 {
		return m[1]
	}
	return ""
}

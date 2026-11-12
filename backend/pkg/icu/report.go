// Package icu разбирает брокерский «Звіт торговця» ІСU (PDF с текстовым слоем): движение денег и облигаций за период.
//
// Отчёт — единственный источник данных брокерского счёта: ни ввода журнала руками, ни сверки «призраков».
// В отчёте нет IBAN и нет ставки/даты погашения облигаций: счёт получает ключ из номера договора,
// а привязка к лицу идёт обычным путём — через алиас по имени файла.
//
// Всё, что парсер не знает по образцам, не угадывается, а даёт явную ошибку (неизвестный вид операции,
// непустой раздел корпоративных операций). Итоги раздела 1 служат контрольными суммами: отчёт, где операции
// не сходятся с остатками, не импортируется.
package icu

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"economic/pkg/pdftext"
)

// Виды потоков — те же строки, что и у журнала счёта в базе.
const (
	KindDeposit  = "deposit"  // «Зарахування ГК»: деньги пришли на счёт (+)
	KindWithdraw = "withdraw" // деньги ушли со счёта (−)
	KindBuy      = "buy"      // купили облигации (−), ISIN + штуки
	KindSell     = "sell"     // продали (+)
)

// Flow — одна запись журнала счёта.
type Flow struct {
	ID     string  // стабильный ключ: повторная загрузка того же (или перекрывающегося) отчёта не создаёт дублей
	Time   int64   // unix; в отчёте только дата — берётся полдень по местному времени
	Kind   string  // deposit | withdraw | buy | sell
	Amount float64 // изменение свободных средств, со знаком
	ISIN   string
	Qty    float64
	Note   string
}

// Totals — строка раздела 1: остаток на начало, поступления, расход, остаток на конец.
type Totals struct{ Start, In, Out, End float64 }

// Position — движение штук облигации за период (раздел 1).
type Position struct {
	ISIN string
	Totals
}

// Report — разобранный отчёт.
type Report struct {
	Contract     string    // «БО-261021-0058» — ключ счёта (в отчёте нет ни IBAN, ни карты)
	ContractDate time.Time // дата договора = дата открытия счёта
	From, To     time.Time
	Cash         Totals // UAH
	Positions    []Position
	Flows        []Flow
}

// ChecksumError — операции отчёта не сходятся с его же остатками.
type ChecksumError struct{ Detail string }

func (e *ChecksumError) Error() string {
	return "звіт не збігається з власними залишками: " + e.Detail
}

var (
	reDate     = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}$`)
	reISIN     = regexp.MustCompile(`^UA\d{10}$`)
	reInt      = regexp.MustCompile(`^\d+$`)
	reMoney    = regexp.MustCompile(`^\d+,\d{2}$`)
	reGroup    = regexp.MustCompile(`^\d{1,3}$`)
	reTail     = regexp.MustCompile(`^\d{3}(,\d{2})?$`)
	rePeriod   = regexp.MustCompile(`за\s+період:\s*(\d{2}\.\d{2}\.\d{4})\s*р?\.?\s*-\s*(\d{2}\.\d{2}\.\d{4})`)
	reContract = regexp.MustCompile(`(БО-\d+-\d+)\s+від\s+(\d{2}\.\d{2}\.\d{4})`)
	reNonTrade = regexp.MustCompile(`^(\d{2}\.\d{2}\.\d{4})\s+(\D+?)\s+(\d[\d ]*,\d{2})\s+([A-Z]{3})$`)
	reSection  = regexp.MustCompile(`^РОЗДІЛ\s+(\d)\b`)
)

// IsReport — это брокерский отчёт ICU? Смотрит только первую страницу.
func IsReport(raw []byte) bool {
	if !bytes.HasPrefix(raw, []byte("%PDF")) {
		return false
	}
	text, err := pdftext.Run(raw, "-l", "1", "-layout")
	if err != nil {
		return false
	}
	return strings.Contains(text, "ЗВІТ ТОРГОВЦЯ") && strings.Contains(text, "РОЗДІЛ 1")
}

// ParseReport разбирает PDF отчёта.
func ParseReport(raw []byte) (*Report, error) {
	bbox, err := pdftext.Run(raw, "-bbox")
	if err != nil {
		return nil, err
	}
	pages := pdftext.ParseBBox(bbox)
	if len(pages) == 0 {
		return nil, fmt.Errorf("у PDF немає текстового шару")
	}
	return ParseWords(pages)
}

// ParseWords разбирает слова страниц (отдельно от pdftotext — для тестов на обезличенных фикстурах).
func ParseWords(pages [][]pdftext.Word) (*Report, error) {
	lines := buildLines(pages)
	rep := &Report{}

	// шапка
	for _, l := range lines {
		t := l.text()
		if m := rePeriod.FindStringSubmatch(t); m != nil && rep.From.IsZero() {
			rep.From, _ = time.ParseInLocation("02.01.2006", m[1], time.Local)
			rep.To, _ = time.ParseInLocation("02.01.2006", m[2], time.Local)
		}
		if m := reContract.FindStringSubmatch(t); m != nil && rep.Contract == "" {
			rep.Contract = m[1]
			rep.ContractDate, _ = time.ParseInLocation("02.01.2006", m[2], time.Local)
		}
	}
	if rep.Contract == "" {
		return nil, fmt.Errorf("у звіті не знайдено номер генерального договору")
	}
	if rep.From.IsZero() {
		return nil, fmt.Errorf("у звіті не знайдено період")
	}

	// разделы
	type span struct{ from, to int }
	sec := map[int]span{}
	var order []int
	for i, l := range lines {
		if m := reSection.FindStringSubmatch(l.text()); m != nil {
			n, _ := strconv.Atoi(m[1])
			sec[n] = span{from: i + 1, to: len(lines)}
			order = append(order, n)
		}
	}
	for k, n := range order {
		if k+1 < len(order) {
			s := sec[n]
			s.to = indexOfSection(lines, order[k+1])
			sec[n] = s
		}
	}
	for _, n := range []int{1, 2, 4, 5} {
		if _, ok := sec[n]; !ok {
			return nil, fmt.Errorf("у звіті немає розділу %d", n)
		}
	}

	if err := parseSection1(rep, lines[sec[1].from:sec[1].to]); err != nil {
		return nil, err
	}
	if err := parseOrders(rep, lines[sec[2].from:sec[2].to]); err != nil {
		return nil, err
	}
	if err := parseNonTrade(rep, lines[sec[4].from:sec[4].to]); err != nil {
		return nil, err
	}
	for _, l := range lines[sec[5].from:sec[5].to] {
		if strings.ContainsAny(l.text(), "0123456789") {
			return nil, fmt.Errorf("розділ 5 (корпоративні операції: купон, погашення) не порожній: формат ще не підтверджено зразком — %q", l.text())
		}
	}

	sort.SliceStable(rep.Flows, func(i, j int) bool { return rep.Flows[i].Time < rep.Flows[j].Time })
	if err := rep.check(); err != nil {
		return nil, err
	}
	return rep, nil
}

func indexOfSection(lines []line, n int) int {
	for i, l := range lines {
		if m := reSection.FindStringSubmatch(l.text()); m != nil {
			if v, _ := strconv.Atoi(m[1]); v == n {
				return i
			}
		}
	}
	return len(lines)
}

// ───────────────────────── строки и числа из слов ─────────────────────────

type line struct{ words []pdftext.Word }

func (l line) text() string {
	parts := make([]string, len(l.words))
	for i, w := range l.words {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
}

// buildLines группирует слова в строки по вертикали (страницы подряд), внутри строки — слева направо.
func buildLines(pages [][]pdftext.Word) []line {
	var out []line
	for _, ws := range pages {
		sorted := append([]pdftext.Word(nil), ws...)
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].YC() < sorted[j].YC() })
		var cur []pdftext.Word
		var y float64
		flush := func() {
			if len(cur) == 0 {
				return
			}
			sort.SliceStable(cur, func(i, j int) bool { return cur[i].X0 < cur[j].X0 })
			out = append(out, line{words: cur})
			cur = nil
		}
		for _, w := range sorted {
			if len(cur) > 0 && math.Abs(w.YC()-y) > 2.5 {
				flush()
			}
			if len(cur) == 0 {
				y = w.YC()
			}
			cur = append(cur, w)
		}
		flush()
	}
	return out
}

// num — число из одного или нескольких слов («5 152,14» приходит как «5» и «152,14»).
type num struct {
	val     float64
	isMoney bool
	xc      float64
}

// numbers склеивает соседние слова-группы разрядов в числа; нечисловые слова пропускает.
func numbers(ws []pdftext.Word) []num {
	var out []num
	for i := 0; i < len(ws); i++ {
		t := ws[i].Text
		if !reInt.MatchString(t) && !reMoney.MatchString(t) {
			continue
		}
		x0, x1, text := ws[i].X0, ws[i].X1, t
		for reGroup.MatchString(text) && i+1 < len(ws) && reTail.MatchString(ws[i+1].Text) && ws[i+1].X0-x1 < 6 {
			i++
			text += ws[i].Text
			x1 = ws[i].X1
		}
		v, err := strconv.ParseFloat(strings.Replace(text, ",", ".", 1), 64)
		if err != nil {
			continue
		}
		out = append(out, num{val: v, isMoney: strings.Contains(text, ","), xc: (x0 + x1) / 2})
	}
	return out
}

func moneyOf(s string) float64 {
	s = strings.NewReplacer(" ", "", " ", "", " ", "").Replace(s)
	v, _ := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	return v
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func noon(date string) (int64, error) {
	t, err := time.ParseInLocation("02.01.2006", date, time.Local)
	if err != nil {
		return 0, err
	}
	return t.Add(12 * time.Hour).Unix(), nil
}

// ───────────────────────── раздел 1: остатки ─────────────────────────

func parseSection1(rep *Report, ls []line) error {
	// колонки — по заголовку; пустые ячейки в тексте не видны, поэтому число относим к ближайшей колонке
	heads := map[string]float64{}
	for _, l := range ls {
		for _, w := range l.words {
			switch w.Text {
			case "початок", "Надходження", "Видаток", "кінець":
				if _, ok := heads[w.Text]; !ok {
					heads[w.Text] = w.XC()
				}
			}
		}
	}
	cols := []float64{heads["початок"], heads["Надходження"], heads["Видаток"], heads["кінець"]}
	for _, c := range cols {
		if c == 0 {
			return fmt.Errorf("розділ 1: не знайдено заголовок колонок")
		}
	}
	nearest := func(x float64) int {
		best := 0
		for i, c := range cols {
			if math.Abs(c-x) < math.Abs(cols[best]-x) {
				best = i
			}
		}
		return best
	}
	left := cols[0] - 45 // дальше — числа, ближе — подпись строки

	read := func(l line) (label string, t Totals, found bool) {
		var lab []string
		var data []pdftext.Word
		for _, w := range l.words {
			if w.X1 <= left {
				lab = append(lab, w.Text)
			} else {
				data = append(data, w)
			}
		}
		vals := [4]float64{}
		for _, n := range numbers(data) {
			vals[nearest(n.xc)] = n.val
			found = true
		}
		return strings.Join(lab, " "), Totals{vals[0], vals[1], vals[2], vals[3]}, found
	}

	for _, l := range ls {
		label, t, found := read(l)
		if strings.HasPrefix(label, "Винагорода") {
			break
		}
		switch {
		case label == "UAH":
			rep.Cash = t
		case strings.HasPrefix(label, "ISIN:"):
			isin := strings.TrimSpace(strings.TrimPrefix(label, "ISIN:"))
			if !reISIN.MatchString(isin) {
				return fmt.Errorf("розділ 1: некоректний ISIN %q", isin)
			}
			rep.Positions = append(rep.Positions, Position{ISIN: isin, Totals: t})
		case found && len(label) == 3 && strings.ToUpper(label) == label && label != "UAH" && !strings.ContainsAny(label, "0123456789"):
			return fmt.Errorf("розділ 1: валюта %s не підтримується (лише UAH)", label)
		}
	}
	return nil
}

// ───────────────────────── раздел 2: сделки ─────────────────────────

func parseOrders(rep *Report, ls []line) error {
	for i, l := range ls {
		var orderNo, side, isin string
		isinAt := -1
		for k, w := range l.words {
			switch {
			case strings.HasPrefix(w.Text, "ЗМФ-"):
				orderNo = w.Text
			case w.Text == "Купівля" || w.Text == "Продаж":
				side = w.Text
			case reISIN.MatchString(w.Text) && isinAt < 0:
				isin, isinAt = w.Text, k
			}
		}
		if orderNo == "" {
			continue
		}
		if side == "" || isinAt < 0 {
			return fmt.Errorf("розділ 2: замовлення %s — не вдалося розібрати вид послуги чи ISIN", orderNo)
		}
		if side != "Купівля" && side != "Продаж" {
			return fmt.Errorf("розділ 2: замовлення %s — невідомий вид послуги %q", orderNo, side)
		}
		// дата виконання — вторым рядком ячейки «Дата замовлення» (строка ниже, слева)
		date := ""
		if i+1 < len(ls) {
			for _, w := range ls[i+1].words {
				if reDate.MatchString(w.Text) {
					date = w.Text
					break
				}
			}
		}
		if date == "" {
			return fmt.Errorf("розділ 2: замовлення %s — не знайдено дату виконання", orderNo)
		}
		ts, err := noon(date)
		if err != nil {
			return fmt.Errorf("розділ 2: замовлення %s — дата %q: %w", orderNo, date, err)
		}
		ns := numbers(l.words[isinAt+1:])
		if len(ns) < 3 || ns[0].isMoney || !ns[1].isMoney || !ns[2].isMoney {
			return fmt.Errorf("розділ 2: замовлення %s — не вдалося розібрати кількість, ціну, суму", orderNo)
		}
		// «Курс» и «Винагорода» в образце — прочерки; любое число там — формат, не подтверждённый зразком
		if len(ns) > 3 {
			return fmt.Errorf("розділ 2: замовлення %s — є курс/винагорода, формат ще не підтверджено зразком", orderNo)
		}
		qty, sum := ns[0].val, round2(ns[2].val)
		f := Flow{ID: "ord:" + orderNo, Time: ts, ISIN: isin, Qty: qty, Amount: -sum, Kind: KindBuy, Note: orderNo}
		if side == "Продаж" {
			f.Kind, f.Amount = KindSell, sum
		}
		rep.Flows = append(rep.Flows, f)
	}
	return nil
}

// ───────────────────────── раздел 4: неторговые операции ─────────────────────────

func parseNonTrade(rep *Report, ls []line) error {
	seen := map[string]int{}
	for _, l := range ls {
		m := reNonTrade.FindStringSubmatch(l.text())
		if m == nil {
			continue
		}
		var kind string
		switch strings.TrimSpace(m[2]) {
		case "Зарахування ГК":
			kind = KindDeposit
		default:
			return fmt.Errorf("розділ 4: невідомий вид операції %q — формат ще не підтверджено зразком", strings.TrimSpace(m[2]))
		}
		if m[4] != "UAH" {
			return fmt.Errorf("розділ 4: валюта %s не підтримується (лише UAH)", m[4])
		}
		ts, err := noon(m[1])
		if err != nil {
			return err
		}
		amt := round2(moneyOf(m[3]))
		key := fmt.Sprintf("op|%s|%s|%d", m[1], kind, int64(math.Round(amt*100)))
		seen[key]++
		rep.Flows = append(rep.Flows, Flow{ID: fmt.Sprintf("%s|%d", key, seen[key]), Time: ts, Kind: kind, Amount: amt, Note: strings.TrimSpace(m[2])})
	}
	return nil
}

// ───────────────────────── контроль ─────────────────────────

func near(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func (r *Report) check() error {
	if !near(r.Cash.Start+r.Cash.In-r.Cash.Out, r.Cash.End) {
		return &ChecksumError{fmt.Sprintf("гроші: %.2f + %.2f − %.2f ≠ %.2f", r.Cash.Start, r.Cash.In, r.Cash.Out, r.Cash.End)}
	}
	var net float64
	qty := map[string]float64{}
	for _, f := range r.Flows {
		net += f.Amount
		switch f.Kind {
		case KindBuy:
			qty[f.ISIN] += f.Qty
		case KindSell:
			qty[f.ISIN] -= f.Qty
		}
	}
	if !near(round2(net), round2(r.Cash.In-r.Cash.Out)) {
		return &ChecksumError{fmt.Sprintf("операції дають %.2f ₴, а розділ 1 — %.2f ₴ (надходження − видаток)", net, r.Cash.In-r.Cash.Out)}
	}
	for _, p := range r.Positions {
		if !near(p.Start+p.In-p.Out, p.End) {
			return &ChecksumError{fmt.Sprintf("%s: %.0f + %.0f − %.0f ≠ %.0f шт.", p.ISIN, p.Start, p.In, p.Out, p.End)}
		}
		if !near(qty[p.ISIN], p.In-p.Out) {
			return &ChecksumError{fmt.Sprintf("%s: операції дають %.0f шт., а розділ 1 — %.0f", p.ISIN, qty[p.ISIN], p.In-p.Out)}
		}
		delete(qty, p.ISIN)
	}
	for isin, q := range qty {
		if q != 0 {
			return &ChecksumError{fmt.Sprintf("%s є в операціях, але немає в розділі 1", isin)}
		}
	}
	return nil
}

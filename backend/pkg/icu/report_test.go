package icu

import (
	"os"
	"strings"
	"testing"
	"time"

	"economic/pkg/pdftext"
)

// Фикстура — обезличенный bbox реального отчёта (имя, РНОКПП и номер договора заменены).
func fixture(t *testing.T, mutate func(string) string) [][]pdftext.Word {
	t.Helper()
	b, err := os.ReadFile("testdata/report_bbox.xml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if mutate != nil {
		s = mutate(s)
	}
	return pdftext.ParseBBox(s)
}

func TestParseRealShapedReport(t *testing.T) {
	rep, err := ParseWords(fixture(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Contract != "БО-261021-0000" || rep.ContractDate.Format("2006-01-02") != "2026-10-21" {
		t.Fatalf("договор: %q %v", rep.Contract, rep.ContractDate)
	}
	if rep.From.Format("2006-01-02") != "2026-09-11" || rep.To.Format("2006-01-02") != "2026-11-10" {
		t.Fatalf("период: %v — %v", rep.From, rep.To)
	}
	// пустые ячейки остатков = 0; поступления и расход разнесены по колонкам
	if rep.Cash != (Totals{Start: 0, In: 5152.14, Out: 5152.14, End: 0}) {
		t.Fatalf("деньги: %+v", rep.Cash)
	}
	want := map[string]Totals{"UA4000239081": {In: 3, End: 3}, "UA4000239131": {In: 2, End: 2}}
	if len(rep.Positions) != 2 {
		t.Fatalf("позиции: %+v", rep.Positions)
	}
	for _, p := range rep.Positions {
		if p.Totals != want[p.ISIN] {
			t.Errorf("%s: %+v", p.ISIN, p.Totals)
		}
	}

	var dep, buy float64
	var buys []Flow
	for _, f := range rep.Flows {
		switch f.Kind {
		case KindDeposit:
			dep += f.Amount
		case KindBuy:
			buy += f.Amount
			buys = append(buys, f)
		}
	}
	if !near(dep, 5152.14) || !near(buy, -5152.14) || len(rep.Flows) != 6 {
		t.Fatalf("потоки: пополнения %.2f покупки %.2f всего %d", dep, buy, len(rep.Flows))
	}
	// дата сделки — дата исполнения (строкой ниже даты заказа): заказ 05.11 исполнен 06.11
	var last Flow
	for _, f := range buys {
		if f.Note == "ЗМФ-261105-2320" {
			last = f
		}
	}
	if last.ISIN != "UA4000239081" || last.Qty != 2 || last.Amount != -2071.12 {
		t.Fatalf("сделка 05.11: %+v", last)
	}
	if got := timeDay(last.Time); got != "2026-11-06" {
		t.Fatalf("дата исполнения: %s", got)
	}
	// id стабильны: повторная загрузка не создаёт дублей
	again, _ := ParseWords(fixture(t, nil))
	for i := range rep.Flows {
		if rep.Flows[i].ID != again.Flows[i].ID {
			t.Fatalf("id нестабилен: %s / %s", rep.Flows[i].ID, again.Flows[i].ID)
		}
	}
}

func TestReportMustMatchItsOwnTotals(t *testing.T) {
	// правим сумму только в разделе 4 (последнее вхождение «071,12»): операции перестают сходиться с разделом 1
	_, err := ParseWords(fixture(t, func(s string) string {
		i := strings.LastIndex(s, ">071,12<")
		return s[:i] + ">072,12<" + s[i+len(">071,12<"):]
	}))
	if _, ok := err.(*ChecksumError); !ok {
		t.Fatalf("ждали ChecksumError, получили %v", err)
	}
}

func TestUnknownOperationIsAnErrorNotAGuess(t *testing.T) {
	_, err := ParseWords(fixture(t, func(s string) string {
		i := strings.LastIndex(s, ">Зарахування<") // последнее вхождение — раздел 4
		return s[:i] + ">Списання<" + s[i+len(">Зарахування<"):]
	}))
	if err == nil || !strings.Contains(err.Error(), "невідомий вид операції") {
		t.Fatalf("ждали ошибку про неизвестную операцию, получили %v", err)
	}
}

func TestNotAReport(t *testing.T) {
	if IsReport([]byte("not a pdf")) {
		t.Fatal("текст не отчёт")
	}
	if _, err := ParseWords(nil); err == nil {
		t.Fatal("пустой документ должен давать ошибку")
	}
}

func timeDay(ts int64) string { return time.Unix(ts, 0).In(time.Local).Format("2006-01-02") }

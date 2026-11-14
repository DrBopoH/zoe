package raif

import (
	"strings"
	"testing"
)

const csvA = `Виписка по картці
Рахунок: UA583003350000000026001234567
Картка: 4149****7777
Баланс на кінець періоду: 1 234,56
Дата і час здійснення операції;Номер картки;Категорія;Деталі операції;MCC;Сума у валюті рахунку;Комісія
10.03.2025 12:00:00;4149****7777;;Переказ з картки: Ivanenko Petro;4829;-5410,00;59,10
11.03.2025 09:30:00;4149****7777;;Кава;5814;-54,10;0,00
11.03.2025 09:30:00;4149****7777;;Кава;5814;-54,10;0,00
12.03.2025 10:00:00;4149****7777;;Зарахування: Іван Петренко;4829;1000,00;0,00
`

func TestParse(t *testing.T) {
	r, err := ParseCSV(strings.NewReader(csvA))
	if err != nil {
		t.Fatal(err)
	}
	if r.IBAN != "UA583003350000000026001234567" || r.CardNumber != "4149****7777" || r.Balance != 1234.56 {
		t.Fatalf("%+v", r)
	}
	tx := r.Transactions
	if len(tx) != 4 {
		t.Fatalf("n=%d", len(tx))
	}
	if tx[0].Amount != -546910 || tx[0].OperationAmount != -541000 || tx[0].CommissionRate != 5910 || tx[0].CounterName != "Ivanenko Petro" {
		t.Fatalf("%+v", tx[0])
	}
	if tx[1].Amount != -5410 { // 54.10 раньше давало 5409
		t.Fatalf("cents: %d", tx[1].Amount)
	}
	if tx[1].ID == tx[2].ID {
		t.Fatal("identical ops in one minute must get different ids")
	}
	// сдвиг строк / другая выгрузка не меняет ID
	shifted := strings.Replace(csvA, "Виписка по картці\n", "Виписка по картці\n\nСформовано 01.04.2025\n", 1)
	r2, err := ParseCSV(strings.NewReader(shifted))
	if err != nil {
		t.Fatal(err)
	}
	for i := range tx {
		if tx[i].ID != r2.Transactions[i].ID {
			t.Fatalf("id unstable at %d", i)
		}
	}
	// Киев, а не TZ сервера: 10.03.2025 12:00 EET = 10:00 UTC
	if tx[0].Time != 1741600800 {
		t.Fatalf("tz: %d", tx[0].Time)
	}
}

// Разбор выписки по частям: каждая часть проверяется отдельно, чтобы поломка показывала, где именно.
func TestParserParts(t *testing.T) {
	if d, err := detectDelimiter("шапка, с запятой\nДата і час;Сума;Деталі операції\n"); err != nil || d != ';' {
		t.Fatalf("разделитель по строке заголовка, а не шапки: %q %v", d, err)
	}
	if d, _ := detectDelimiter("Дата,Опис,Сума\n"); d != ',' {
		t.Fatalf("запятая: %q", d)
	}
	if _, err := detectDelimiter("\n  \n"); err == nil {
		t.Fatal("пустой файл должен давать ошибку")
	}

	col := mapColumns([]string{"Дата", "x", "Номер картки", "Опис", "MCC", "Сума у валюті рахунку", "Комісія"})
	if col != (columns{date: 0, desc: 3, mcc: 4, amount: 5, fee: 6, card: 2}) {
		t.Fatalf("колонки: %+v", col)
	}
	if got := mapColumns([]string{"a", "b"}); got != defaultColumns() {
		t.Fatalf("без названий — по умолчанию: %+v", got)
	}

	if counterOf("Переказ: Іван Петренко") != "Іван Петренко" || counterOf("Зарахування: 01.11.2026") != "" || counterOf("без двоеточия") != "" {
		t.Fatal("имя контрагента из описания")
	}
	if iban, card, bal := statementHead("UA583003350000026001234567890\nКартка: 4441****1234\nБаланс на кінець періоду: −1 234,50"); iban == "" || card != "4441****1234" || bal != -1234.5 {
		t.Fatalf("шапка: %q %q %v", iban, card, bal)
	}
}

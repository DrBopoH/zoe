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

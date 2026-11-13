package binance

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

// История C2C-ордеров (Orders → P2P → Export): отдельная выписка того же счёта. В журнале транзакций P2P виден только как
// «+88,85 USDT» в Funding, без гривневой стороны; здесь у каждого ордера есть сумма в фиате, цена, комиссия и время создания.
// Читаются ВСЕ ордера как есть (в том числе отменённые, Status сохраняется): что считать покупкой, решает потребитель.

// P2POrder — один C2C-ордер.
type P2POrder struct {
	ID           string  `json:"id"`           // «Номер ордера»
	Side         string  `json:"side"`         // Buy / Sell
	Asset        string  `json:"asset"`        // монета (USDT)
	Fiat         string  `json:"fiat"`         // валюта платежа (UAH)
	Total        float64 `json:"total"`        // сумма в фиате
	Price        float64 `json:"price"`        // цена монеты в фиате
	Qty          float64 `json:"qty"`          // количество монеты (до комиссии)
	Rate         float64 `json:"rate"`         // «Обмінний курс» (в выписке бывает пусто)
	Fee          float64 `json:"fee"`          // комиссия мейкера + тейкера, в монете
	Counterparty string  `json:"counterparty"` // контрагент
	Status       string  `json:"status"`       // Completed / Cancelled …
	T            int64   `json:"t"`            // время создания, мс
}

// Credit — сколько монеты реально пришло на кошелёк (покупка) или ушло с него (продажа): количество минус комиссия.
func (o P2POrder) Credit() float64 { return o.Qty - o.Fee }

// Done — ордер завершён.
func (o P2POrder) Done() bool { return strings.EqualFold(o.Status, "Completed") }

var p2pHeads = []string{"номер ордера", "order number", "номер заказа", "order no.", "order no"}

// IsP2P — файл похож на историю C2C-ордеров (первая колонка — номер ордера).
func IsP2P(raw []byte) bool {
	h := headOf(raw)
	for _, s := range p2pHeads {
		if h == s {
			return true
		}
	}
	return false
}

const p2pCols = 13 // номер, тип, актив, фиат, сумма, цена, количество, курс, комиссия мейкера, комиссия тейкера, контрагент, статус, время

func p2pNum(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
}

// ParseP2P разбирает историю C2C-ордеров. Время в файле — в поясе из имени файла «…(UTC+2)…» (см. StatementZone).
func ParseP2P(r io.Reader, loc *time.Location) ([]P2POrder, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if !IsP2P(raw) {
		return nil, errors.New("не похоже на историю P2P-ордеров Binance (ожидается «Номер ордера, Тип ордера, Актив, …»)")
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
	var out []P2POrder
	for _, row := range rows[1:] {
		if len(row) < p2pCols {
			continue
		}
		o := P2POrder{
			ID: strings.TrimSpace(row[0]), Side: strings.TrimSpace(row[1]), Asset: strings.TrimSpace(row[2]), Fiat: strings.TrimSpace(row[3]),
			Counterparty: strings.TrimSpace(row[10]), Status: strings.TrimSpace(row[11]),
		}
		if o.ID == "" {
			continue
		}
		var nums [6]float64
		for i, col := range []int{4, 5, 6, 7, 8, 9} {
			if nums[i], err = p2pNum(row[col]); err != nil {
				return nil, errors.New("не разобрать число «" + row[col] + "» в ордере " + o.ID)
			}
		}
		o.Total, o.Price, o.Qty, o.Rate, o.Fee = nums[0], nums[1], nums[2], nums[3], nums[4]+nums[5]
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(row[12]), loc)
		if err != nil {
			return nil, errors.New("не разобрать время «" + row[12] + "» в ордере " + o.ID)
		}
		o.T = t.UnixMilli()
		out = append(out, o)
	}
	if len(out) == 0 {
		return nil, errors.New("в истории нет ордеров")
	}
	return out, nil
}

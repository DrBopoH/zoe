// Package market — публичные рыночные данные по облигациям: ОВДП (реестр депозитария и результаты
// аукционов Минфина через открытые данные НБУ) и казначейские бумаги США (FiscalData). Ключей нет,
// ничего личного не уходит наружу: запросы только на чтение публичных справочников.
//
// Формат ответов внешних API проверяется по их документации, а не по живому ответу, поэтому разбор терпимый:
// поле ищется по списку возможных имён, пустое и "null" — «нет значения», незнакомые поля игнорируются.
// Запись без ISIN/CUSIP пропускается и считается в Result.Skipped.
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	SourceNBU = "nbu" // ОВДП: реестр депозитария НБУ + результаты аукционов
	SourceUST = "ust" // US Treasury: аукционы FiscalData

	DefaultNBUBase     = "https://bank.gov.ua"
	DefaultFREDBase    = "https://fred.stlouisfed.org"
	DefaultBinanceBase = "https://api.binance.com"
	DefaultFNGBase     = "https://api.alternative.me"
	DefaultUSTBase     = "https://api.fiscaldata.treasury.gov/services/api/fiscal_service"

	maxBody = 32 << 20 // реестр ОВДП — единицы мегабайт; больше — это уже не он
)

// Security — одна облигация из публичного источника.
type Security struct {
	Source      string  `json:"source"`
	ID          string  `json:"id"` // ISIN (nbu) или CUSIP (ust)
	Name        string  `json:"name"`
	Currency    string  `json:"currency"`               // UAH | USD | EUR
	Maturity    string  `json:"maturity"`               // YYYY-MM-DD
	Issued      string  `json:"issued,omitempty"`       // дата размещения/выпуска
	CouponRate  float64 `json:"coupon_rate"`            // годовая ставка, %; 0 — бескупонная или неизвестна
	Nominal     float64 `json:"nominal"`                // номинал одной бумаги в её валюте
	Yield       float64 `json:"yield,omitempty"`        // доходность последнего аукциона, %
	LastAuction string  `json:"last_auction,omitempty"` // дата последнего аукциона
}

// Result — итог одного обновления источника.
type Result struct {
	Source  string `json:"source"`
	Fetched int    `json:"fetched"`           // записей получено и разобрано
	Skipped int    `json:"skipped,omitempty"` // записей без идентификатора или не облигаций
	Error   string `json:"error,omitempty"`
}

// Client ходит в НБУ и FiscalData. Базовые адреса и часы подменяются в тестах.
type Client struct {
	HTTP    *http.Client
	NBUBase string
	USTBase string
	// FREDBase — адрес FRED (ставка ФРС, инфляция США, доходность Treasury); публичный CSV без ключа
	FREDBase string
	// BinanceBase — публичные свечи Binance (без ключа, без подписи); FNGBase — индекс страха и жадности крипторынка
	BinanceBase string
	FNGBase     string
	Now         func() time.Time
	// USTYears — за сколько лет назад брать аукционы США (по умолчанию 3: хватает на все бумаги, что ещё в обращении
	// у 2-летних и 3-летних выпусков; долгие бумаги подтянутся при следующих обновлениях по мере появления).
	USTYears int
	// AuctionDays — сколько последних дней перебирать в результатах аукционов ОВДП (по умолчанию 14).
	AuctionDays int
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}, NBUBase: DefaultNBUBase, USTBase: DefaultUSTBase, FREDBase: DefaultFREDBase, BinanceBase: DefaultBinanceBase, FNGBase: DefaultFNGBase, Now: time.Now}
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	h := c.HTTP
	if h == nil {
		h = http.DefaultClient
	}
	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("ответ больше %d МБ", maxBody>>20)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}

// ───────────────────────── разбор полей ─────────────────────────

type record map[string]any

// str — первое непустое значение из перечисленных ключей (регистр ключа не важен).
func (r record) str(keys ...string) string {
	for _, k := range keys {
		for rk, v := range r {
			if !strings.EqualFold(rk, k) || v == nil {
				continue
			}
			var s string
			switch x := v.(type) {
			case string:
				s = strings.TrimSpace(x)
			case float64:
				s = strconv.FormatFloat(x, 'f', -1, 64)
			default:
				continue
			}
			if s != "" && !strings.EqualFold(s, "null") {
				return s
			}
		}
	}
	return ""
}

// num — число из строки или числа: "16,44", "16.44", 16.44; пустое, "null" и мусор → 0.
func (r record) num(keys ...string) float64 {
	s := strings.ReplaceAll(strings.ReplaceAll(r.str(keys...), " ", ""), ",", ".")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// date — YYYY-MM-DD из ISO, DD.MM.YYYY или YYYYMMDD (с хвостом «T…» или временем); не разобралось → "".
func (r record) date(keys ...string) string { return normDate(r.str(keys...)) }

func normDate(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "T "); i > 0 {
		s = s[:i]
	}
	for _, layout := range []string{"2006-01-02", "02.01.2006", "20060102"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

// records разбирает JSON-массив объектов; {"data":[…]} (FiscalData) тоже понимает.
func records(body []byte) ([]record, error) {
	var arr []record
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var wrap struct {
		Data []record `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, fmt.Errorf("ответ не JSON-массив: %w", err)
	}
	return wrap.Data, nil
}

func upperCur(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "980":
		return "UAH"
	case "840":
		return "USD"
	case "978":
		return "EUR"
	case "":
		return "UAH"
	}
	return s
}

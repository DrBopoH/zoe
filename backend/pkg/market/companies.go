package market

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Список компаний для раздела «Акции» (этап F1). Источники подтверждены зондом Дока 17.11.2026 (probe8), оба публичные и без ключа:
// Nasdaq screener — одним запросом тикер, название, цена, капитализация, страна, сектор и год IPO (около 2,3 МБ);
// SEC company_tickers_exchange — тикер → CIK (номер в EDGAR: по нему в F3 берётся отчётность) и биржа.
// В список попадает компания, которая есть в обоих и у неё известны цена и капитализация: так выпадают фонды и бумаги без отчётности.
// Закрытые компании (Anthropic, OpenAI, SpaceX) сюда не попадают сами — публичных котировок у них нет. Ничего не вводится руками.
const (
	SourceCompanies = "companies"

	DefaultNasdaqBase = "https://api.nasdaq.com"
	DefaultSECBase    = "https://www.sec.gov"

	nasdaqScreenerPath = "/api/screener/stocks?tableonly=true&limit=25&offset=0&download=true"
	secTickersPath     = "/files/company_tickers_exchange.json"

	// SEC просит представляться «название контакт@домен»: без User-Agent и с неправдоподобным контактом (адрес на localhost, скобки) отвечает HTTP 403.
	// Проверенная зондом форма (probe8, 200 OK) — имя и адрес на публичном домене.
	secUserAgent = "Zoe-app dok@example.com"
)

// Company — компания из списка. Капитализация и цена — на момент опроса (история — этап F2).
type Company struct {
	Ticker    string  `json:"ticker"` // в записи SEC и Yahoo: класс акций через дефис (BRK-B)
	Name      string  `json:"name"`
	Exchange  string  `json:"exchange"` // Nasdaq | NYSE | …
	CIK       int     `json:"cik"`      // номер в EDGAR (SEC)
	Sector    string  `json:"sector,omitempty"`
	Industry  string  `json:"industry,omitempty"`
	Country   string  `json:"country,omitempty"`
	IPOYear   int     `json:"ipo_year,omitempty"`
	MarketCap float64 `json:"market_cap"` // $
	Price     float64 `json:"price"`      // $
}

// Companies опрашивает оба источника. Сбой любого — пустой список и ошибка в итоге: по половине данных (без CIK или без капитализации) список не нужен.
func (c *Client) Companies(ctx context.Context, rep Reporter) ([]Company, Result) {
	res := Result{Source: SourceCompanies}
	if c.NasdaqBase == "" || c.SECBase == "" {
		return nil, res
	}
	rep.report(10, "SEC · тикеры и CIK")
	sec, err := c.secTickers(ctx)
	if err != nil {
		res.Error = "SEC: " + err.Error()
		return nil, res
	}
	rep.report(50, "Nasdaq · капитализация")
	rows, err := c.nasdaqRows(ctx)
	if err != nil {
		res.Error = "Nasdaq: " + err.Error()
		return nil, res
	}
	out, skipped := joinCompanies(rows, sec)
	res.Fetched, res.Skipped = len(out), skipped
	if len(out) == 0 {
		res.Error = "списки не сошлись: ни одной компании с CIK и капитализацией"
	}
	return out, res
}

type secTicker struct {
	cik      int
	name     string
	ticker   string
	exchange string
}

// secTickers — реестр SEC: массив data из строк [cik, название, тикер, биржа].
func (c *Client) secTickers(ctx context.Context) ([]secTicker, error) {
	body, err := c.getHeaders(ctx, c.SECBase+secTickersPath, map[string]string{"Accept": "application/json", "User-Agent": secUserAgent})
	if err != nil {
		return nil, err
	}
	var r struct {
		Data [][]any `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("ответ не разобран: %w", err)
	}
	out := make([]secTicker, 0, len(r.Data))
	for _, row := range r.Data {
		if len(row) < 4 {
			continue
		}
		cik, _ := row[0].(float64)
		name, _ := row[1].(string)
		ticker, _ := row[2].(string)
		ex, _ := row[3].(string)
		if cik > 0 && ticker != "" {
			out = append(out, secTicker{int(cik), name, ticker, ex})
		}
	}
	return out, nil
}

type nasdaqRow struct {
	Symbol, Name, LastSale, MarketCap, Country, IPOYear, Sector, Industry string
}

// nasdaqRows — таблица screener: data.rows, все поля строками («$123.45», «3500000000.00»).
func (c *Client) nasdaqRows(ctx context.Context) ([]nasdaqRow, error) {
	// Nasdaq отшивает клиента без браузерного User-Agent
	body, err := c.getHeaders(ctx, c.NasdaqBase+nasdaqScreenerPath, map[string]string{"Accept": "application/json", "User-Agent": "Mozilla/5.0"})
	if err != nil {
		return nil, err
	}
	var r struct {
		Data struct {
			Rows []struct {
				Symbol    string `json:"symbol"`
				Name      string `json:"name"`
				LastSale  string `json:"lastsale"`
				MarketCap string `json:"marketCap"`
				Country   string `json:"country"`
				IPOYear   string `json:"ipoyear"`
				Sector    string `json:"sector"`
				Industry  string `json:"industry"`
			} `json:"rows"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("ответ не разобран: %w", err)
	}
	out := make([]nasdaqRow, 0, len(r.Data.Rows))
	for _, x := range r.Data.Rows {
		out = append(out, nasdaqRow{x.Symbol, x.Name, x.LastSale, x.MarketCap, x.Country, x.IPOYear, x.Sector, x.Industry})
	}
	return out, nil
}

// tickerKey — тикер без различий записи класса акций: BRK/B, BRK.B и BRK-B — одна бумага.
func tickerKey(s string) string {
	return strings.NewReplacer("/", "-", ".", "-", " ", "-").Replace(strings.ToUpper(strings.TrimSpace(s)))
}

// moneyNum — «$1,234.50» → 1234.5; пусто и мусор — 0.
func moneyNum(s string) float64 {
	s = strings.NewReplacer("$", "", ",", "", " ", "").Replace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}

// joinCompanies сводит строки Nasdaq с реестром SEC по тикеру. skipped — строки Nasdaq без пары в SEC или без цены и капитализации.
// Порядок — по капитализации, крупные первыми; одинаковые — по тикеру.
func joinCompanies(rows []nasdaqRow, sec []secTicker) ([]Company, int) {
	byKey := make(map[string]secTicker, len(sec))
	for _, s := range sec {
		if _, dup := byKey[tickerKey(s.ticker)]; !dup { // один тикер может числиться за двумя CIK: берём первый (основной выпуск)
			byKey[tickerKey(s.ticker)] = s
		}
	}
	seen := map[string]bool{}
	var out []Company
	skipped := 0
	for _, r := range rows {
		key := tickerKey(r.Symbol)
		s, ok := byKey[key]
		capUSD, price := moneyNum(r.MarketCap), moneyNum(r.LastSale)
		if !ok || capUSD <= 0 || price <= 0 || seen[key] {
			skipped++
			continue
		}
		seen[key] = true
		ipo, _ := strconv.Atoi(strings.TrimSpace(r.IPOYear))
		name := strings.TrimSpace(r.Name)
		if name == "" {
			name = s.name
		}
		out = append(out, Company{Ticker: s.ticker, Name: name, Exchange: s.exchange, CIK: s.cik, Sector: strings.TrimSpace(r.Sector), Industry: strings.TrimSpace(r.Industry), Country: strings.TrimSpace(r.Country), IPOYear: ipo, MarketCap: capUSD, Price: price})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MarketCap != out[j].MarketCap {
			return out[i].MarketCap > out[j].MarketCap
		}
		return out[i].Ticker < out[j].Ticker
	})
	return out, skipped
}

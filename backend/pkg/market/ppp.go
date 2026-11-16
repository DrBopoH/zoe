package market

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Покупательная способность: данные для расчёта на экране (формулы живут на фронте, utils/ppp.js). Все источники подтверждены зондом 16.11.2026 (probe10):
// Всемирный банк — годовые ряды для 15 стран одним запросом на показатель: индекс цен FP.CPI.TOTL (2010 = 100), средний годовой курс PA.NUS.FCRF (местной валюты за доллар),
// PPP-коэффициент частного потребления PA.NUS.PRVT.PP (местной валюты за международный доллар), доход на душу по PPP и номинальный (NY.GDP.PCAP.PP.CD, NY.GDP.PCAP.CD).
// У еврозоны (EMU) нет индекса цен и PPP-коэффициента — они остаются у Германии. Данные годовые, последний год — 2025 (США по индексу цен — 2024):
// годовые точки ставятся на середину года (1 июля) — это годовое среднее, и интерполяция между ними честнее, чем «конец года».
// Индекс цен США помесячно — FRED CPIAUCSL (до последнего месяца): им считается «что стоил доллар раньше».
// Big Mac Index (The Economist, открытые данные на GitHub): цена в долларах и отклонение от доллара в процентах, дважды в год.
const (
	SourcePPP = "ppp"

	SerPPPUSCPI    = "ppp_us_cpi" // индекс потребительских цен США помесячно (CPIAUCSL, 1982–84 = 100)
	pppCPIPrefix   = "ppp_cpi_"   // индекс цен страны, 2010 = 100 (годовой)
	pppFXPrefix    = "ppp_fx_"    // курс к доллару: местной валюты за 1 USD, среднее за год
	pppConvPrefix  = "ppp_conv_"  // PPP-коэффициент: местной валюты за 1 международный доллар
	pppGDPPrefix   = "ppp_gdp_"   // ВВП на душу по PPP, международных $
	pppGDPNPrefix  = "ppp_gdpn_"  // ВВП на душу номинальный, $
	bmUSDPrefix    = "bm_usd_"    // цена Биг Мака в долларах по рыночному курсу
	bmRawPrefix    = "bm_raw_"    // отклонение цены Биг Мака от американской, %: минус — валюта недооценена
	bigMacPath     = "/TheEconomist/big-mac-data/master/output-data/big-mac-full-index.csv"
	pppFirstYear   = 1960
	pppLastYearMax = 2030
)

// pppCountries — страны набора: код реестра (country.go) и ISO-3 Всемирного банка. de — Германия: запасной вариант для еврозоны.
var pppCountries = []struct{ cc, iso3 string }{
	{"ua", "UKR"}, {"us", "USA"}, {"ez", "EMU"}, {"gb", "GBR"}, {"ch", "CHE"}, {"jp", "JPN"}, {"cn", "CHN"}, {"ru", "RUS"},
	{"pl", "POL"}, {"tr", "TUR"}, {"no", "NOR"}, {"se", "SWE"}, {"ca", "CAN"}, {"eg", "EGY"}, {"kr", "KOR"}, {"au", "AUS"}, {"br", "BRA"}, {"sa", "SAU"}, {"ae", "ARE"}, {"in", "IND"}, {"de", "DEU"},
}

// pppIndicators — показатели Всемирного банка; noEZ — у еврозоны (EMU) ряда нет.
var pppIndicators = []struct {
	code, prefix string
	noEZ         bool
}{
	{"FP.CPI.TOTL", pppCPIPrefix, true},
	{"PA.NUS.FCRF", pppFXPrefix, false},
	{"PA.NUS.PRVT.PP", pppConvPrefix, true},
	{"NY.GDP.PCAP.PP.CD", pppGDPPrefix, false},
	{"NY.GDP.PCAP.CD", pppGDPNPrefix, false},
}

// bigMacISO — страны Биг Мака (у еврозоны код EUZ); Германии нет — отдельного ряда по ней не бывает.
var bigMacISO = map[string]string{
	"UKR": "ua", "USA": "us", "EUZ": "ez", "GBR": "gb", "CHE": "ch", "JPN": "jp", "CHN": "cn",
	"RUS": "ru", "POL": "pl", "TUR": "tr", "NOR": "no", "SWE": "se", "CAN": "ca", "EGY": "eg", "KOR": "kr", "AUS": "au", "BRA": "br", "SAU": "sa", "ARE": "ae", "IND": "in",
}

// pppSeries — полный перечень рядов источника (для каталога).
var pppSeries = func() []string {
	out := []string{SerPPPUSCPI}
	for _, ind := range pppIndicators {
		for _, c := range pppCountries {
			if c.cc == "ez" && ind.noEZ {
				continue
			}
			out = append(out, ind.prefix+c.cc)
		}
	}
	var cc []string
	for _, v := range bigMacISO {
		cc = append(cc, v)
	}
	sort.Strings(cc)
	for _, c := range cc {
		out = append(out, bmUSDPrefix+c, bmRawPrefix+c)
	}
	return out
}()

// PPP — данные покупательной способности. Сбой части (показатель Всемирного банка, FRED, Big Mac) не трогает остальное, ошибка — в Result.Error.
func (c *Client) PPP(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourcePPP}
	var out []SeriesPoint
	var errs []string
	if c.WBBase != "" {
		for i, ind := range pppIndicators {
			rep.report(5+i*14, ind.code)
			pts, err := c.wbPPP(ctx, ind.code, ind.prefix, ind.noEZ)
			if err != nil {
				errs = append(errs, err.Error())
			}
			out = append(out, pts...)
		}
	}
	if c.FREDBase != "" {
		rep.report(78, "индекс цен США")
		r := c.fredAll(ctx, rep, []fredJob{{ID: "CPIAUCSL", Note: "индекс цен США", Series: SerPPPUSCPI}})
		out = append(out, toPoints(SerPPPUSCPI, r.Obs["CPIAUCSL"])...)
		res.Skipped, res.Tail = r.Skipped, r.Tail
		errs = append(errs, r.Errs...)
	}
	if c.ShillerBase != "" {
		rep.report(90, "Big Mac Index")
		pts, err := c.bigMac(ctx)
		if err != nil {
			errs = append(errs, err.Error())
		}
		out = append(out, pts...)
	}
	res.Fetched, res.Error = len(out), strings.Join(errs, "; ")
	return out, res
}

// wbPPP — один показатель сразу по всем странам набора (страны через «;»). Годовая точка ставится на 1 июля.
func (c *Client) wbPPP(ctx context.Context, code, prefix string, noEZ bool) ([]SeriesPoint, error) {
	isoToCC := map[string]string{}
	var list []string
	for _, p := range pppCountries {
		if p.cc == "ez" && noEZ {
			continue
		}
		isoToCC[p.iso3] = p.cc
		list = append(list, p.iso3)
	}
	url := fmt.Sprintf("%s/v2/country/%s/indicator/%s?format=json&per_page=3000&date=%d:%d", strings.TrimRight(c.WBBase, "/"), strings.Join(list, ";"), code, pppFirstYear, pppLastYearMax)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("Всемирный банк %s: %w", code, err)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil || len(raw) < 2 {
		return nil, fmt.Errorf("Всемирный банк %s: неверный ответ (начало: %q)", code, head160(body))
	}
	var rows []struct {
		ISO   string   `json:"countryiso3code"`
		Date  string   `json:"date"`
		Value *float64 `json:"value"`
	}
	if err := json.Unmarshal(raw[1], &rows); err != nil {
		return nil, fmt.Errorf("Всемирный банк %s: неверный ответ (начало: %q)", code, head160(body))
	}
	var out []SeriesPoint
	for _, r := range rows {
		cc, ok := isoToCC[r.ISO]
		if !ok || r.Value == nil || len(r.Date) != 4 || !(*r.Value > 0) || math.IsInf(*r.Value, 0) {
			continue
		}
		out = append(out, SeriesPoint{Series: prefix + cc, Date: r.Date + "-07-01", Value: *r.Value})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("Всемирный банк %s: нет значений", code)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Series != out[j].Series {
			return out[i].Series < out[j].Series
		}
		return out[i].Date < out[j].Date
	})
	return out, nil
}

// bigMac — цена Биг Мака в долларах и отклонение от американской цены (колонки dollar_price и USD_raw в долях → проценты).
func (c *Client) bigMac(ctx context.Context) ([]SeriesPoint, error) {
	body, err := c.get(ctx, strings.TrimRight(c.ShillerBase, "/")+bigMacPath)
	if err != nil {
		return nil, fmt.Errorf("Big Mac: %w", err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(body))).ReadAll()
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("Big Mac: пустой или неверный ответ")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.TrimSpace(h)] = i
	}
	id, ip, ir, iv := col["iso_a3"], col["dollar_price"], col["USD_raw"], col["date"]
	if id == 0 || ip == 0 || ir == 0 {
		return nil, fmt.Errorf("Big Mac: нет колонок iso_a3, dollar_price, USD_raw")
	}
	num := func(r []string, i int) (float64, bool) {
		if i >= len(r) {
			return 0, false
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(r[i]), 64)
		return v, err == nil && !math.IsNaN(v)
	}
	var out []SeriesPoint
	for _, r := range rows[1:] {
		if len(r) <= id || len(r) <= iv {
			continue
		}
		cc, ok := bigMacISO[strings.TrimSpace(r[id])]
		d := normDate(r[iv])
		if !ok || d == "" {
			continue
		}
		if p, ok := num(r, ip); ok && p > 0 {
			out = append(out, SeriesPoint{Series: bmUSDPrefix + cc, Date: d, Value: round(p, 3)})
		}
		if v, ok := num(r, ir); ok {
			out = append(out, SeriesPoint{Series: bmRawPrefix + cc, Date: d, Value: round(v*100, 1)})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("Big Mac: нет подходящих строк")
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Series != out[j].Series {
			return out[i].Series < out[j].Series
		}
		return out[i].Date < out[j].Date
	})
	return out, nil
}

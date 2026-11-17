package market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Отчётность SEC через EDGAR frames (этапы F2b и F3). Подтверждено зондом probe9 (17.11.2026): один запрос «показатель × период» отдаёт значение для
// всех компаний сразу (2,7–6,4 тысячи, до 1 МБ, около 0,5 с) — вместо десяти тысяч companyfacts по 4 МБ. Данные есть с 2009–2010 годов (до XBRL — единицы компаний).
// Период — календарный кадр SEC: CY2024Q4I (на конец квартала), CY2023 (за год). Значение у каждой компании своё по дате («end»), ближайшей к периоду.
//
// Число акций в отчётности НЕ поправлено на сплиты (у Apple за 2020Q2 там 4,28 млрд при нынешних 15 млрд), а цены Yahoo поправлены; сплиты берутся из Yahoo
// (StockHistory), и CapSeries приводит акции к сегодняшней базе.
const (
	DefaultEDGARBase = "https://data.sec.gov"

	TagSharesBalance = "us-gaap/CommonStockSharesOutstanding"   // акции на балансе: самое широкое покрытие (4,8–6 тысяч компаний)
	TagSharesCover   = "dei/EntityCommonStockSharesOutstanding" // обложка отчёта: запасной вариант, если баланс акции не раскрывает
	framesFirstYear  = 2009                                     // XBRL у крупных компаний с 2009, у остальных с 2011: раньше кадры почти пустые
	framesFilingLag  = 45 * 24 * time.Hour                      // отчёты за квартал приходят до 45 дней после его конца
	framesAnnualLag  = 90 * 24 * time.Hour                      // годовой отчёт приходит до 60–90 дней после конца года
	framesRecent     = 2                                        // последние кварталы перекачиваются, пока в них дописываются поздние отчёты
)

// FrameRequest — один кадр: показатель (таксономия/тег), единица и период.
type FrameRequest struct {
	Tag    string `json:"tag"`    // us-gaap/CommonStockSharesOutstanding
	Unit   string `json:"unit"`   // shares | USD
	Period string `json:"period"` // CY2024Q4I
	Recent bool   `json:"-"`      // один из последних периодов: перекачивается, пока SEC принимает отчёты
}

// Key — ключ кадра в хранилище.
func (r FrameRequest) Key() string { return r.Tag + "|" + r.Period }

// FrameFact — значение показателя у одной компании (по CIK): дата, на которую оно относится, и само число.
type FrameFact struct {
	CIK int     `json:"cik"`
	End string  `json:"end"`
	Val float64 `json:"val"`
}

// quarterPeriods — кадры «на конец квартала» с 2009Q4, по которым отчёты уже должны были прийти.
func quarterPeriods(now time.Time) []string {
	var periods []string
	for y := framesFirstYear; y <= now.Year(); y++ {
		for q := 1; q <= 4; q++ {
			end := time.Date(y, time.Month(q*3)+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
			if y == framesFirstYear && q < 4 || end.Add(framesFilingLag).After(now) {
				continue
			}
			periods = append(periods, fmt.Sprintf("CY%dQ%dI", y, q))
		}
	}
	return periods
}

// annualPeriods — годовые кадры CY2023 с 2009 года, закончившиеся и успевшие получить годовые отчёты.
func annualPeriods(now time.Time) []string {
	var periods []string
	for y := framesFirstYear; y <= now.Year(); y++ {
		if time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC).Add(framesAnnualLag).After(now) {
			continue
		}
		periods = append(periods, fmt.Sprintf("CY%d", y))
	}
	return periods
}

// StockFrameRequests — все кадры, которые нужны сейчас: акции, капитал и активы по закончившимся кварталам с 2009 года, выручка и прибыль по годам.
func StockFrameRequests(now time.Time) []FrameRequest {
	var out []FrameRequest
	add := func(periods []string, unit string, tags ...string) {
		for i, p := range periods {
			recent := i >= len(periods)-framesRecent
			for _, tag := range tags {
				out = append(out, FrameRequest{Tag: tag, Unit: unit, Period: p, Recent: recent})
			}
		}
	}
	quarters := quarterPeriods(now)
	add(quarters, "shares", TagSharesBalance, TagSharesCover)
	add(quarters, "USD", TagEquity, TagAssets)
	add(annualPeriods(now), "USD", append(append([]string{}, RevenueChain...), TagNetIncome)...)
	return out
}

// StockFrame — один кадр целиком.
func (c *Client) StockFrame(ctx context.Context, req FrameRequest) ([]FrameFact, error) {
	if c.EDGARBase == "" {
		return nil, fmt.Errorf("источник EDGAR отключён")
	}
	u := fmt.Sprintf("%s/api/xbrl/frames/%s/%s/%s.json", strings.TrimRight(c.EDGARBase, "/"), req.Tag, req.Unit, req.Period)
	body, err := c.getHeaders(ctx, u, map[string]string{"Accept": "application/json", "User-Agent": secUserAgent})
	var he *HTTPError
	if errors.As(err, &he) && he.Code == http.StatusNotFound {
		return nil, nil // у SEC нет такого кадра: тег ещё не существовал (RevenueFromContract… — с 2018) или за период никто не отчитался. Это «данных нет», а не сбой: пустой кадр сохраняется и больше не запрашивается
	}
	if err != nil {
		return nil, fmt.Errorf("EDGAR %s %s: %w", req.Tag, req.Period, err)
	}
	var r struct {
		Data []struct {
			CIK int     `json:"cik"`
			End string  `json:"end"`
			Val float64 `json:"val"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("EDGAR %s %s: неверный ответ (начало: %q)", req.Tag, req.Period, head160(body))
	}
	out := make([]FrameFact, 0, len(r.Data))
	for _, d := range r.Data {
		if d.CIK > 0 && d.End != "" && d.Val != 0 {
			out = append(out, FrameFact{CIK: d.CIK, End: d.End, Val: d.Val})
		}
	}
	return out, nil
}

// StockFacts качает кадры по одному и отдаёт каждый в save сразу. Кадр, которого у SEC нет (404), сохраняется пустым;
// об ошибке источника сообщается, если без значений осталась половина кадров и больше.
func (c *Client) StockFacts(ctx context.Context, rep Reporter, todo []FrameRequest, save func(FrameRequest, []FrameFact) error) Result {
	res := Result{Source: SourceStockFacts}
	if c.EDGARBase == "" {
		return res
	}
	var errs []string
	for i, req := range todo {
		if ctx.Err() != nil {
			return res
		}
		rep.report(i*100/len(todo), req.Period)
		facts, err := c.StockFrame(ctx, req)
		if err == nil {
			err = save(req, facts)
		}
		if err != nil {
			if ctx.Err() != nil {
				return res
			}
			res.Skipped++
			if len(errs) < 3 {
				errs = append(errs, err.Error())
			}
			continue
		}
		res.Fetched += len(facts)
	}
	if res.Skipped > 0 && res.Skipped*2 >= len(todo) {
		res.Error = fmt.Sprintf("без значений %d из %d кадров: %s", res.Skipped, len(todo), strings.Join(errs, "; "))
	}
	return res
}

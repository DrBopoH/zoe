package market

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Курсы НБУ (bank.gov.ua/NBU_Exchange/exchange_site): официальный курс валюты и золота к гривне по дням.
// Форма ответа проверена живым curl 12.11.2026: exchangedate «ДД.ММ.ГГГГ», rate_per_unit — гривен за единицу
// (у золота — за тройскую унцию), история доллара с 1996 года, окна по 10 лет сервер принимает.
const (
	SourceFX = "fx"

	SerUSDUAH = "usd_uah" // гривен за 1 доллар США
	SerEURUAH = "eur_uah" // гривен за 1 евро
	SerXAUUAH = "xau_uah" // гривен за 1 тройскую унцию золота

	fxFirstYear = 1996
	fxWindow    = 10
)

// До деноминации (2 сентября 1996, 100 000 карбованцев = 1 гривна) НБУ отдаёт курс в карбованцах (179900 вместо 1,799).
var redenomination = time.Date(1996, 9, 2, 0, 0, 0, 0, time.UTC)

var fxCodes = []struct{ code, series string }{
	{"usd", SerUSDUAH},
	{"eur", SerEURUAH},
	{"xau", SerXAUUAH},
}

// FXNBU — курсы USD, EUR и золота к гривне за всю историю. Сбой одного ряда не убивает остальные: что скачалось — возвращается.
func (c *Client) FXNBU(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceFX}
	var out []SeriesPoint
	var errs []string
	now := c.now()
	windows := 0
	for y := fxFirstYear; y <= now.Year(); y += fxWindow {
		windows++
	}
	step, total := 0, windows*len(fxCodes)
	for _, fc := range fxCodes {
		var got []SeriesPoint
		failed := false
		for y := fxFirstYear; y <= now.Year(); y += fxWindow {
			rep.report(5+step*90/total, fmt.Sprintf("%s %d–%d", strings.ToUpper(fc.code), y, min(y+fxWindow-1, now.Year())))
			step++
			end := time.Date(min(y+fxWindow-1, now.Year()), 12, 31, 0, 0, 0, 0, time.UTC)
			if end.After(now) {
				end = now
			}
			pts, sk, err := c.fxWindow(ctx, fc.code, fc.series, time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), end)
			res.Skipped += sk
			if err != nil {
				errs = append(errs, err.Error())
				failed = true
				break
			}
			got = append(got, pts...)
		}
		// ряд с дыркой в середине хуже старого целого: при сбое окна ряд не отдаётся, в кеше остаётся прежний
		if !failed {
			sort.Slice(got, func(i, j int) bool { return got[i].Date < got[j].Date })
			out = append(out, got...)
		}
	}
	rep.report(95, "")
	res.Fetched = len(out)
	res.Error = strings.Join(errs, "; ")
	return out, res
}

func (c *Client) fxWindow(ctx context.Context, code, series string, from, to time.Time) ([]SeriesPoint, int, error) {
	url := fmt.Sprintf("%s/NBU_Exchange/exchange_site?start=%s&end=%s&valcode=%s&sort=exchangedate&order=asc&json",
		strings.TrimRight(c.NBUBase, "/"), from.Format("20060102"), to.Format("20060102"), code)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, 0, fmt.Errorf("курс %s %d–%d: %w", strings.ToUpper(code), from.Year(), to.Year(), err)
	}
	recs, err := records(body)
	if err != nil {
		return nil, 0, fmt.Errorf("курс %s %d–%d: %w", strings.ToUpper(code), from.Year(), to.Year(), err)
	}
	skipped := 0
	var out []SeriesPoint
	for _, r := range recs {
		d := r.date("exchangedate")
		v := r.num("rate_per_unit")
		if v == 0 {
			if u := r.num("units"); u > 0 {
				v = r.num("rate") / u
			}
		}
		if d == "" || v <= 0 {
			skipped++
			continue
		}
		if t, err := time.Parse("2006-01-02", d); err == nil && t.Before(redenomination) {
			v /= 100000
		}
		out = append(out, SeriesPoint{Series: series, Date: d, Value: v})
	}
	return out, skipped, nil
}

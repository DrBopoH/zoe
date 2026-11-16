package market

import (
	"context"
	"strings"
)

// Реальные эффективные курсы валют (REER, широкая корзина, помесячно): индекс стоимости валюты к корзине партнёров с поправкой на инфляцию.
// Растёт — валюта дорожает в реальном выражении (экспорт страны дороже), падает — дешевеет. Источник — BIS через зеркало FRED, ряды RB<CC>BIS;
// все 12 подтверждены зондом Дока 15.11.2026 (данные до 2026-07, с 1994). Египта и Украины в наборе нет — их карточки без этой строки.
const SerREERPrefix = "reer_"

// reerFRED — страна набора и её ряд FRED (для еврозоны BIS использует код XM).
var reerFRED = []struct{ cc, fred string }{
	{"us", "RBUSBIS"}, {"ez", "RBXMBIS"}, {"gb", "RBGBBIS"}, {"ch", "RBCHBIS"}, {"jp", "RBJPBIS"}, {"cn", "RBCNBIS"},
	{"ru", "RBRUBIS"}, {"pl", "RBPLBIS"}, {"tr", "RBTRBIS"}, {"no", "RBNOBIS"}, {"se", "RBSEBIS"}, {"ca", "RBCABIS"},
	{"kr", "RBKRBIS"}, {"tw", "RBTWBIS"}, {"au", "RBAUBIS"}, {"br", "RBBRBIS"},
}

// SerREER — id ряда реального эффективного курса страны: reer_us, reer_ez, …
func SerREER(cc string) string { return SerREERPrefix + cc }

var reerSeries = func() []string {
	out := make([]string, len(reerFRED))
	for i, r := range reerFRED {
		out[i] = SerREER(r.cc)
	}
	return out
}()

// REER — реальные эффективные курсы 12 валют (FRED). Сбой одного ряда не трогает остальные.
func (c *Client) REER(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceFX}
	if c.FREDBase == "" {
		return nil, res
	}
	jobs := make([]fredJob, len(reerFRED))
	for i, r := range reerFRED {
		jobs[i] = fredJob{ID: r.fred, Note: "реальный эффективный курс " + r.cc}
	}
	r := c.fredAll(ctx, rep, jobs)
	var out []SeriesPoint
	for _, x := range reerFRED {
		out = append(out, toPoints(SerREER(x.cc), r.Obs[x.fred])...)
	}
	res.Fetched, res.Skipped, res.Tail = len(out), r.Skipped, r.Tail
	res.Error = strings.Join(r.Errs, "; ")
	return out, res
}

// FXWorld — курсы НБУ (валюты и золото) и реальные эффективные курсы BIS. Сбой любой части не трогает другую.
func (c *Client) FXWorld(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	out, res := c.FXNBU(ctx, rep)
	reer, rres := c.REER(ctx, rep)
	out = append(out, reer...)
	res = mergeResult(res, rres)
	res.Fetched = len(out)
	return out, res
}

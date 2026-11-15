package market

import (
	"context"
	"strings"
)

// Пакеты Польши и Турции. Все источники подтверждены зондами (не выведены по аналогии):
// ставки — BIS WS_CBPOL (PL, TR есть), инфляция — Евростат HICP (prc_hicp_minr, geo PL и TR), безработица — FRED LRHUTTTT (PL, TR, до 2026-08),
// доходность 10 лет Польши — FRED IRLTLT01PL (до 2026-09); курсы злотого и лиры к гривне — НБУ (источник «fx»).
// Не взяли: доходность 10 лет Турции (в FRED/OECD нет, Евростат её не отдаёт), ВВП, резервы, ставку Турции из ЦБ (BIS отдаёт её сам).
const (
	SourcePL = "pl"
	SourceTR = "tr"

	SerPLPolicy = "pl_policy" // ставка NBP (BIS), % — только даты смены значения
	SerPLCPI    = "pl_cpi"    // инфляция HICP г/г, %, помесячно (Евростат)
	SerPLUnrate = "pl_u"      // безработица, %, помесячно (FRED, зеркало ОЭСР)
	SerPLY10    = "pl_y10"    // доходность 10 лет, %, помесячно (FRED, зеркало ОЭСР)

	SerTRPolicy = "tr_policy" // ставка ЦБ Турции (BIS), % — только даты смены значения
	SerTRCPI    = "tr_cpi"    // инфляция HICP г/г, %, помесячно (Евростат)
	SerTRUnrate = "tr_u"      // безработица, %, помесячно (FRED, зеркало ОЭСР)
)

var (
	plSeries = []string{SerPLPolicy, SerPLCPI, SerPLUnrate, SerPLY10}
	trSeries = []string{SerTRPolicy, SerTRCPI, SerTRUnrate}
)

// fredSeries — ряд FRED, который кладётся в кеш под своим id серии.
type fredSeries struct{ id, series, note string }

// fredPack — пакет страны из двух частей: ряды FRED (параллельно) и ряды BIS/Евростата. Сбой части и отдельного ряда не трогает остальное.
func (c *Client) fredPack(ctx context.Context, rep Reporter, source string, fs []fredSeries, jobs []packJob) ([]SeriesPoint, Result) {
	res := Result{Source: source}
	var out []SeriesPoint
	if c.FREDBase != "" && len(fs) > 0 {
		fj := make([]fredJob, len(fs))
		for i, f := range fs {
			fj[i] = fredJob{ID: f.id, Note: f.note}
		}
		r := c.fredAll(ctx, rep, fj)
		for _, f := range fs {
			out = append(out, toPoints(f.series, r.Obs[f.id])...)
		}
		res.Fetched, res.Skipped, res.Tail, res.Error = len(out), r.Skipped, r.Tail, strings.Join(r.Errs, "; ")
	}
	if len(jobs) > 0 {
		pts, r := c.runJobs(ctx, rep, source, jobs)
		out = append(out, pts...)
		res = mergeResult(res, r)
	}
	res.Fetched = len(out)
	return out, res
}

// countryJobs — ставка из BIS и инфляция из Евростата для страны с кодом iso2, если соответствующие источники включены.
func (c *Client) countryJobs(iso2, policyID, cpiID string) []packJob {
	var jobs []packJob
	if c.BISBase != "" {
		jobs = append(jobs, bisPolicy(policyID, iso2))
	}
	if c.EurostatBase != "" {
		jobs = append(jobs, eurostatJob(cpiID, "prc_hicp_minr/M.RCH_A.TOTAL."+iso2))
	}
	return jobs
}

// Poland — ряды Польши: ставка NBP (BIS), инфляция (Евростат), безработица и доходность 10 лет (FRED).
func (c *Client) Poland(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.fredPack(ctx, rep, SourcePL, []fredSeries{
		{"LRHUTTTTPLM156S", SerPLUnrate, "безработица Польши"},
		{"IRLTLT01PLM156N", SerPLY10, "доходность 10 лет Польши"},
	}, c.countryJobs("PL", SerPLPolicy, SerPLCPI))
}

// Turkey — ряды Турции: ставка ЦБ (BIS), инфляция (Евростат), безработица (FRED).
func (c *Client) Turkey(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	return c.fredPack(ctx, rep, SourceTR, []fredSeries{
		{"LRHUTTTTTRM156S", SerTRUnrate, "безработица Турции"},
	}, c.countryJobs("TR", SerTRPolicy, SerTRCPI))
}

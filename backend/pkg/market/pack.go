package market

import (
	"context"
	"fmt"
	"strings"
)

// runJobs опрашивает ряды пакета по очереди. Сбой одного ряда не убивает остальные: что скачалось — возвращается, ошибки — в Result.Error.
func (c *Client) runJobs(ctx context.Context, rep Reporter, source string, jobs []packJob) ([]SeriesPoint, Result) {
	res := Result{Source: source}
	var out []SeriesPoint
	var errs []string
	for i, j := range jobs {
		rep.report(5+i*90/len(jobs), j.series)
		tryGeos := []string{""}
		if j.geos {
			tryGeos = ezGeos
		}
		get := j.get
		if get == nil {
			get = (*Client).sdmxGet
		}
		var pts []SeriesPoint
		var err error
		for _, g := range tryGeos {
			if pts, err = get(c, ctx, j.url(c, g), j.series); err == nil {
				break
			}
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", j.series, err))
			continue
		}
		out = append(out, j.post(pts)...)
	}
	rep.report(95, "")
	res.Fetched, res.Error = len(out), strings.Join(errs, "; ")
	return out, res
}

func bisURL(c *Client, flowKey string) string {
	return strings.TrimRight(c.BISBase, "/") + "/api/v2/data/dataflow/BIS/" + flowKey + "?format=csv"
}

// bisPolicy — ключевая ставка центробанка страны из BIS (WS_CBPOL, помесячно). Единственный ряд ставок, подтверждённый зондом сразу для всех стран набора.
func bisPolicy(id, iso2 string) packJob {
	return packJob{series: id, url: func(c *Client, _ string) string { return bisURL(c, "WS_CBPOL/1.0/M."+iso2) }, post: changePoints}
}

// eurostatJob — ряд Евростата: набор и ключ вида «une_rt_m/M.SA.TOTAL.PC_ACT.T.CH».
func eurostatJob(id, flowKey string) packJob {
	return packJob{series: id, url: func(c *Client, _ string) string { return eurostatURL(c, flowKey) }, post: ident}
}

package market

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Пакеты Японии и Китая. Япония: ставка и инфляция — BIS, безработица, доходность JGB 10 лет и Nikkei 225 — FRED (все ряды проверены зондом);
// курсы йены и юаня к гривне — в источнике «fx» (НБУ). Китай: ставка и инфляция — BIS.
// Что не взяли (нет подтверждённого источника из Украины, зонд №3): безработица, ВВП, PPI, резервы и доходность 10 лет Китая (PBoC, NBS недоступны, FRED-зеркала пусты).
const (
	SourceJP = "jp"
	SourceCN = "cn"

	SerJPPolicy = "jp_policy" // ставка Банка Японии (BIS), % — только даты смены значения
	SerJPCPI    = "jp_cpi"    // инфляция CPI г/г, % (BIS)
	SerJPUnrate = "jp_u"      // безработица, %, помесячно (FRED, зеркало ОЭСР)
	SerJPY10    = "jp_y10"    // доходность JGB 10 лет, %, помесячно (FRED, зеркало ОЭСР)
	SerJPNikkei = "jp_nikkei" // индекс Nikkei 225, пункты — последнее значение недели (FRED)

	SerCNPolicy = "cn_policy" // ставка Народного банка Китая (BIS), % — только даты смены значения
	SerCNCPI    = "cn_cpi"    // инфляция CPI г/г, % (BIS)

	SerCHY10 = "ch_y10" // доходность 10 лет Швейцарии, %, помесячно (FRED, зеркало ОЭСР)
)

var (
	jpSeries = []string{SerJPPolicy, SerJPCPI, SerJPUnrate, SerJPY10, SerJPNikkei}
	cnSeries = []string{SerCNPolicy, SerCNCPI}
)

// bisCPIJob — инфляция страны из BIS (WS_LONG_CPI, помесячно): годовое изменение в процентах.
func bisCPIJob(id, iso2 string) packJob {
	return packJob{
		series: id,
		url:    func(c *Client, _ string) string { return bisURL(c, "WS_LONG_CPI/1.0/M."+iso2) },
		get: func(c *Client, ctx context.Context, url, series string) ([]SeriesPoint, error) {
			body, err := c.fetchPlain(ctx, url)
			if err != nil {
				return nil, err
			}
			return parseBISCPI(body, series)
		},
		post: ident,
	}
}

// parseBISCPI читает WS_LONG_CPI: на страну приходит несколько рядов (по единице измерения). Годовые изменения в процентах (единица 771) берутся как есть;
// если их нет — считаются из индекса любой другой единицы (к той же дате прошлого года).
func parseBISCPI(body []byte, series string) ([]SeriesPoint, error) {
	rows, err := csvRows(body)
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("неверный ответ BIS (начало: %q, ошибка разбора: %v)", head160(body), err)
	}
	ui, ti, vi := -1, -1, -1
	for i, h := range rows[0] {
		switch strings.TrimSpace(h) {
		case "UNIT_MEASURE":
			ui = i
		case "TIME_PERIOD":
			ti = i
		case "OBS_VALUE":
			vi = i
		}
	}
	if ti < 0 || vi < 0 {
		return nil, fmt.Errorf("нет колонок TIME_PERIOD и OBS_VALUE (начало ответа: %q)", head160(body))
	}
	byUnit := map[string][]SeriesPoint{}
	for _, r := range rows[1:] {
		if len(r) <= ti || len(r) <= vi {
			continue
		}
		unit := ""
		if ui >= 0 && len(r) > ui {
			unit = strings.TrimSpace(r[ui])
		}
		one, err := parseSDMX([]byte("TIME_PERIOD,OBS_VALUE\n"+r[ti]+","+r[vi]+"\n"), series)
		if err != nil {
			continue
		}
		byUnit[unit] = append(byUnit[unit], one...)
	}
	if pts := byUnit["771"]; len(pts) > 0 {
		sort.Slice(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
		return pts, nil
	}
	units := make([]string, 0, len(byUnit))
	for u := range byUnit {
		units = append(units, u)
	}
	sort.Strings(units)
	for _, u := range units {
		var obs []fredObs
		for _, p := range byUnit[u] {
			obs = append(obs, fredObs{Date: p.Date, Value: p.Value})
		}
		sort.Slice(obs, func(i, j int) bool { return obs[i].Date < obs[j].Date })
		if yoy, _ := indexChanges(obs, series, ""); len(yoy) > 0 {
			return yoy, nil
		}
	}
	return nil, fmt.Errorf("в ответе BIS нет ни процентов г/г, ни индекса (начало: %q)", head160(body))
}

// mergeResult склеивает итоги двух частей одного источника (часть FRED и часть SDMX/CSV).
func mergeResult(a, b Result) Result {
	a.Fetched += b.Fetched
	a.Skipped += b.Skipped
	a.Tail = append(a.Tail, b.Tail...)
	switch {
	case a.Error == "":
		a.Error = b.Error
	case b.Error != "":
		a.Error += "; " + b.Error
	}
	return a
}

// Japan — ряды Японии. Сбой части (FRED или BIS) и отдельного ряда не трогает остальное.
func (c *Client) Japan(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceJP}
	var out []SeriesPoint
	if c.FREDBase != "" {
		r := c.fredAll(ctx, rep, []fredJob{{"LRHUTTTTJPM156S", "безработица Японии", ""}, {"IRLTLT01JPM156N", "доходность JGB 10 лет", ""}, {"NIKKEI225", "Nikkei 225", SerJPNikkei}})
		out = append(out, toPoints(SerJPUnrate, r.Obs["LRHUTTTTJPM156S"])...)
		out = append(out, toPoints(SerJPY10, r.Obs["IRLTLT01JPM156N"])...)
		out = append(out, weekly(toPoints(SerJPNikkei, r.Obs["NIKKEI225"]))...)
		res.Fetched, res.Skipped, res.Tail, res.Error = len(out), r.Skipped, r.Tail, strings.Join(r.Errs, "; ")
	}
	if c.BISBase != "" {
		pts, r := c.runJobs(ctx, rep, SourceJP, []packJob{bisPolicy(SerJPPolicy, "JP"), bisCPIJob(SerJPCPI, "JP")})
		out = append(out, pts...)
		res = mergeResult(res, r)
	}
	res.Fetched = len(out)
	return out, res
}

// China — ряды Китая: ставка и инфляция из BIS.
func (c *Client) China(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	if c.BISBase == "" {
		return nil, Result{Source: SourceCN}
	}
	return c.runJobs(ctx, rep, SourceCN, []packJob{bisPolicy(SerCNPolicy, "CN"), bisCPIJob(SerCNCPI, "CN")})
}

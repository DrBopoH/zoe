package market

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Пакеты стран: Великобритания (Банк Англии, ONS) и Швейцария (BIS, Евростат). Источник на страну: своя кнопка «Обновить» и сбой не трогает соседей.
// Что не взяли (нет подтверждённого источника, зонд №3): ВВП Швейцарии, базовая инфляция Великобритании.
const (
	SourceGB = "gb"
	SourceCH = "ch"

	SerGBPolicy = "gb_policy" // ставка Банка Англии (Bank Rate), % — только даты смены значения
	SerGBSonia  = "gb_sonia"  // SONIA — овернайт, % — последнее значение недели
	SerGBY10    = "gb_y10"    // доходность Gilt 10 лет (номинальная par yield), % — последнее значение недели
	SerGBCPI    = "gb_cpi"    // инфляция CPI г/г, %, помесячно (ONS D7G7)
	SerGBUnrate = "gb_u"      // безработица (16+, с учётом сезонности), %, скользящие 3 месяца (ONS MGSX)
	SerGBGDP    = "gb_gdp"    // рост ВВП к предыдущему кварталу, % (ONS IHYQ)

	SerCHPolicy = "ch_policy" // ставка SNB (BIS, помесячно), % — только даты смены значения
	SerCHCPI    = "ch_cpi"    // инфляция HICP г/г, %, помесячно (Евростат)
	SerCHUnrate = "ch_u"      // безработица (с учётом сезонности), %, помесячно (Евростат)
)

var (
	gbSeries = []string{SerGBPolicy, SerGBSonia, SerGBY10, SerGBCPI, SerGBUnrate, SerGBGDP}
	chSeries = []string{SerCHPolicy, SerCHCPI, SerCHUnrate, SerCHY10}
)

// boeURL — суточный ряд базы IADB Банка Англии: csv без оформления (CSVF=TN), даты «02 Jan 2024».
func boeURL(c *Client, code string) string {
	return strings.TrimRight(c.BoEBase, "/") + "/boeapps/database/_iadb-fromshowcolumns.asp?csv.x=yes&Datefrom=01/Jan/1975&Dateto=now&SeriesCodes=" + code + "&CSVF=TN&UsingCodes=Y&VPD=Y&VFD=N"
}

func boeJob(id, code string, post func([]SeriesPoint) []SeriesPoint) packJob {
	return packJob{series: id, url: func(c *Client, _ string) string { return boeURL(c, code) }, get: (*Client).boeGet, post: post}
}

// onsJob — ряд ONS по пути набора: «economy/…/timeseries/d7g7/mm23». quarterly: брать строки кварталов вместо месяцев.
func onsJob(id, path string, quarterly bool) packJob {
	return packJob{
		series: id,
		url: func(c *Client, _ string) string {
			return strings.TrimRight(c.ONSBase, "/") + "/generator?format=csv&uri=/" + path
		},
		get: func(c *Client, ctx context.Context, url, series string) ([]SeriesPoint, error) {
			body, err := c.fetchPlain(ctx, url)
			if err != nil {
				return nil, err
			}
			return parseONS(body, series, quarterly)
		},
		post: ident,
	}
}

func (c *Client) boeGet(ctx context.Context, url, series string) ([]SeriesPoint, error) {
	body, err := c.fetchPlain(ctx, url)
	if err != nil {
		return nil, err
	}
	return parseBoE(body, series)
}

func csvRows(body []byte) ([][]string, error) {
	rd := csv.NewReader(bytes.NewReader(body))
	rd.FieldsPerRecord = -1
	rd.LazyQuotes = true
	return rd.ReadAll()
}

func head160(body []byte) string {
	h := strings.Join(strings.Fields(string(body)), " ")
	if len(h) > 160 {
		h = h[:160]
	}
	return h
}

// parseBoE читает CSV базы IADB: первая строка «DATE,<код>», дальше «02 Jan 2024,5.25». Строки без числа пропускаются.
func parseBoE(body []byte, series string) ([]SeriesPoint, error) {
	rows, err := csvRows(body)
	if err != nil || len(rows) < 2 || !strings.EqualFold(strings.TrimSpace(rows[0][0]), "DATE") {
		return nil, fmt.Errorf("неверный ответ Банка Англии (начало: %q, ошибка разбора: %v)", head160(body), err)
	}
	var out []SeriesPoint
	for _, r := range rows[1:] {
		if len(r) < 2 {
			continue
		}
		d, err := time.Parse("02 Jan 2006", strings.TrimSpace(r[0]))
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(r[1]), 64)
		if err != nil {
			continue
		}
		out = append(out, SeriesPoint{Series: series, Date: d.Format("2006-01-02"), Value: v})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("в ответе Банка Англии нет значений (начало: %q)", head160(body))
	}
	return out, nil
}

var (
	onsMonth   = regexp.MustCompile(`^(\d{4}) ([A-Z]{3})(?:-([A-Z]{3}))?$`)
	onsQuarter = regexp.MustCompile(`^(\d{4}) Q([1-4])$`)
	months     = map[string]int{"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12}
)

// parseONS читает «generator»-CSV ONS: сверху строки-описание («Title», «CDID»…), дальше «метка,значение», где метка — год («2024»), квартал («2024 Q1»)
// или месяц («2024 JAN»; у скользящих рядов — «2024 SEP-NOV»: берётся последний месяц периода). Нужные строки — месячные или квартальные, годовые игнорируются.
func parseONS(body []byte, series string, quarterly bool) ([]SeriesPoint, error) {
	rows, err := csvRows(body)
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("неверный ответ ONS (начало: %q, ошибка разбора: %v)", head160(body), err)
	}
	var out []SeriesPoint
	for _, r := range rows {
		if len(r) < 2 {
			continue
		}
		label := strings.ToUpper(strings.TrimSpace(r[0]))
		v, err := strconv.ParseFloat(strings.TrimSpace(r[1]), 64)
		if err != nil {
			continue
		}
		if quarterly {
			if m := onsQuarter.FindStringSubmatch(label); m != nil {
				q, _ := strconv.Atoi(m[2])
				out = append(out, SeriesPoint{Series: series, Date: fmt.Sprintf("%s-%02d-01", m[1], (q-1)*3+1), Value: v})
			}
			continue
		}
		if m := onsMonth.FindStringSubmatch(label); m != nil {
			mon, ok := months[m[2]]
			if m[3] != "" {
				mon, ok = months[m[3]]
			}
			if ok {
				out = append(out, SeriesPoint{Series: series, Date: fmt.Sprintf("%s-%02d-01", m[1], mon), Value: v})
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("в ответе ONS нет значений (начало: %q)", head160(body))
	}
	return out, nil
}

// UK — ряды Великобритании. Сбой одного ряда не убивает остальные.
func (c *Client) UK(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	var jobs []packJob
	if c.BoEBase != "" {
		jobs = append(jobs, boeJob(SerGBPolicy, "IUDBEDR", changePoints), boeJob(SerGBSonia, "IUDSOIA", weekly), boeJob(SerGBY10, "IUDMNPY", weekly))
	}
	if c.ONSBase != "" {
		jobs = append(jobs,
			onsJob(SerGBCPI, "economy/inflationandpriceindices/timeseries/d7g7/mm23", false),
			onsJob(SerGBUnrate, "employmentandlabourmarket/peoplenotinwork/unemployment/timeseries/mgsx/lms", false),
			onsJob(SerGBGDP, "economy/grossdomesticproductgdp/timeseries/ihyq/qna", true))
	}
	return c.runJobs(ctx, rep, SourceGB, jobs)
}

// Switzerland — ряды Швейцарии: ставка SNB из BIS, инфляция и безработица из Евростата, доходность 10 лет из FRED.
func (c *Client) Switzerland(ctx context.Context, rep Reporter) ([]SeriesPoint, Result) {
	res := Result{Source: SourceCH}
	var out []SeriesPoint
	if c.FREDBase != "" {
		r := c.fredAll(ctx, rep, []fredJob{{"IRLTLT01CHM156N", "доходность 10 лет Швейцарии", ""}})
		out = append(out, toPoints(SerCHY10, r.Obs["IRLTLT01CHM156N"])...)
		res.Fetched, res.Skipped, res.Tail, res.Error = len(out), r.Skipped, r.Tail, strings.Join(r.Errs, "; ")
	}
	var jobs []packJob
	if c.BISBase != "" {
		jobs = append(jobs, bisPolicy(SerCHPolicy, "CH"))
	}
	if c.EurostatBase != "" {
		jobs = append(jobs, eurostatJob(SerCHCPI, "prc_hicp_minr/M.RCH_A.TOTAL.CH"), eurostatJob(SerCHUnrate, "une_rt_m/M.SA.TOTAL.PC_ACT.T.CH"))
	}
	pts, r := c.runJobs(ctx, rep, SourceCH, jobs)
	out = append(out, pts...)
	res = mergeResult(res, r)
	res.Fetched = len(out)
	return out, res
}

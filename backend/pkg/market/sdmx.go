package market

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// sdmxHeaders — заголовки для SDMX-CSV Евростата и ЕЦБ: не стандартный User-Agent Go (Евростат такой отшивает), без сжатия.
var sdmxHeaders = map[string]string{"Accept": "text/csv,*/*", "Accept-Encoding": "identity", "User-Agent": "Mozilla/5.0 (compatible; Zoe personal finance; +local)"}

// sdmxGet скачивает SDMX-CSV (Евростат: format=SDMX-CSV, ЕЦБ: format=csvdata) и разбирает его в ряд series.
// Евростат иногда присылает gzip без заголовка Content-Encoding — Go такое сам не распаковывает: узнаём по сигнатуре 1f 8b.
func (c *Client) sdmxGet(ctx context.Context, url, series string) ([]SeriesPoint, error) {
	body, err := c.getHeaders(ctx, url, sdmxHeaders)
	if err != nil {
		return nil, err
	}
	if len(body) > 2 && body[0] == 0x1f && body[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("ответ gzip не распаковывается: %w", err)
		}
		plain, err := io.ReadAll(io.LimitReader(zr, maxBody+1))
		if err != nil {
			return nil, fmt.Errorf("ответ gzip не распаковывается: %w", err)
		}
		body = plain
	}
	return parseSDMX(body, series)
}

// sdmxDate приводит период SDMX к дате: день «2026-11-12» как есть, месяц «2026-10» → первое число, квартал «2026-Q3» → первое число квартала.
// Год без месяца и всё непонятное — false.
func sdmxDate(per string) (string, bool) {
	per = strings.TrimSpace(per)
	switch {
	case len(per) == 10 && per[4] == '-' && per[7] == '-':
		return per, true
	case len(per) == 7 && per[4] == '-' && per[5] != 'Q':
		return per + "-01", true
	case len(per) == 7 && per[4] == '-' && per[5] == 'Q' && per[6] >= '1' && per[6] <= '4':
		return fmt.Sprintf("%s-%02d-01", per[:4], (int(per[6]-'1'))*3+1), true
	}
	return "", false
}

// parseSDMX читает SDMX-CSV: колонки TIME_PERIOD и OBS_VALUE; даты — по sdmxDate. Строки без значения («n/a») пропускаются.
func parseSDMX(body []byte, series string) ([]SeriesPoint, error) {
	head := func() string { // начало ответа для сообщения об ошибке
		h := strings.Join(strings.Fields(string(body)), " ")
		if len(h) > 160 {
			h = h[:160]
		}
		return h
	}
	rd := csv.NewReader(strings.NewReader(string(body)))
	rd.FieldsPerRecord = -1 // строки SDMX-CSV бывают разной длины (хвост флагов)
	rd.LazyQuotes = true
	rows, err := rd.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("пустой или неверный ответ (начало: %q, ошибка разбора: %v)", head(), err)
	}
	ti, vi := -1, -1
	for i, h := range rows[0] {
		switch strings.TrimSpace(h) {
		case "TIME_PERIOD":
			ti = i
		case "OBS_VALUE":
			vi = i
		}
	}
	if ti < 0 || vi < 0 {
		return nil, fmt.Errorf("нет колонок TIME_PERIOD и OBS_VALUE (начало ответа: %q)", head())
	}
	var out []SeriesPoint
	for _, r := range rows[1:] {
		if len(r) <= ti || len(r) <= vi {
			continue
		}
		d, ok := sdmxDate(r[ti])
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(r[vi]), 64)
		if err != nil {
			continue
		}
		out = append(out, SeriesPoint{Series: series, Date: d, Value: v})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("в ответе нет значений (начало ответа: %q)", head())
	}
	return out, nil
}

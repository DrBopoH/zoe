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

// Инфляция еврозоны: годовое изменение HICP (общий индекс, EA) из SDMX-CSV Евростата, без ключа.
// С февраля 2026 Евростат (и ЕЦБ) закрыли старые наборы (prc_hicp_manr, ICP) на декабре 2025 года и перешли на новый HICP.
// Новый набор prc_hicp_minr с ключом M.RCH_A.TOTAL.EA отдаёт всю историю с 1997 года до последнего месяца (проверено живым curl).
const euPath = "/eurostat/api/dissemination/sdmx/2.1/data/prc_hicp_minr/M.RCH_A.TOTAL.EA?format=SDMX-CSV"

func (c *Client) euroInflation(ctx context.Context) ([]SeriesPoint, error) {
	pts, err := c.eurostatCSV(ctx, strings.TrimRight(c.EurostatBase, "/")+euPath)
	if err != nil {
		return nil, fmt.Errorf("инфляция еврозоны: %w", err)
	}
	return pts, nil
}

// eurostatCSV читает SDMX-CSV Евростата: колонки TIME_PERIOD («2026-10») и OBS_VALUE; даты — первое число месяца.
func (c *Client) eurostatCSV(ctx context.Context, url string) ([]SeriesPoint, error) {
	body, err := c.getHeaders(ctx, url, map[string]string{"Accept": "text/csv,*/*", "Accept-Encoding": "identity", "User-Agent": "Mozilla/5.0 (compatible; Zoe personal finance; +local)"})
	if err != nil {
		return nil, err
	}
	// Евростат иногда присылает gzip без заголовка Content-Encoding — Go такое сам не распаковывает: узнаём по сигнатуре 1f 8b
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
		per := strings.TrimSpace(r[ti])
		if len(per) != 7 || per[4] != '-' {
			continue // не месяц
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(r[vi]), 64)
		if err != nil {
			continue
		}
		out = append(out, SeriesPoint{Series: SerEUCPIYoY, Date: per + "-01", Value: v})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("в ответе нет месячных значений (начало ответа: %q)", head())
	}
	return out, nil
}

package market

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Всемирный банк (api.worldbank.org, без ключа): годовые показатели для стран, которых нет в BIS и Евростате. Данные годовые и с лагом в год-два.

// wbJob — годовой ряд показателя Всемирного банка для страны (ISO-3, например EGY): точка ставится на конец года.
func wbJob(id, iso3, indicator string) packJob {
	return packJob{
		series: id,
		url: func(c *Client, _ string) string {
			return strings.TrimRight(c.WBBase, "/") + "/v2/country/" + iso3 + "/indicator/" + indicator + "?format=json&per_page=200"
		},
		get: func(c *Client, ctx context.Context, url, series string) ([]SeriesPoint, error) {
			body, err := c.get(ctx, url)
			if err != nil {
				return nil, err
			}
			return parseWB(body, series)
		},
		post: ident,
	}
}

// parseWB разбирает ответ [служебная часть, [{"date":"2024","value":28.3}, …]]. Годы без значения (null) пропускаются.
// Ошибку API Всемирный банк тоже отдаёт 200-м ответом с одним элементом-сообщением — это ошибка, а не пустой ряд.
func parseWB(body []byte, series string) ([]SeriesPoint, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil || len(raw) < 2 {
		return nil, fmt.Errorf("неверный ответ Всемирного банка (начало: %q)", head160(body))
	}
	var rows []struct {
		Date  string   `json:"date"`
		Value *float64 `json:"value"`
	}
	if err := json.Unmarshal(raw[1], &rows); err != nil {
		return nil, fmt.Errorf("неверный ответ Всемирного банка (начало: %q)", head160(body))
	}
	var out []SeriesPoint
	for _, r := range rows {
		if r.Value == nil || len(r.Date) != 4 {
			continue
		}
		out = append(out, SeriesPoint{Series: series, Date: r.Date + "-12-01", Value: *r.Value})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("в ответе Всемирного банка нет значений (начало: %q)", head160(body))
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 { // банк отдаёт от новых к старым
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

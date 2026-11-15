package market

import (
	"context"
	"fmt"
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

// eurostatCSV читает SDMX-CSV Евростата как ряд инфляции еврозоны (месячные точки).
func (c *Client) eurostatCSV(ctx context.Context, url string) ([]SeriesPoint, error) {
	return c.sdmxGet(ctx, url, SerEUCPIYoY)
}

package market

import (
	"context"
	"testing"
	"time"
)

func TestFrequencyByAverageStep(t *testing.T) {
	cases := []struct {
		points   int
		from, to string
		want     string
	}{
		{1, "2026-01-01", "2026-01-01", "снимок"},
		{0, "", "", "снимок"},
		{101, "2026-01-01", "2026-04-11", "день"},
		{53, "2025-01-05", "2026-01-04", "неделя"},
		{13, "2025-01-01", "2026-01-01", "месяц"},
		{5, "2025-01-01", "2026-01-01", "квартал"},
		{5, "2020-01-01", "2024-01-01", "год"},
		{3, "2026-02-01", "2026-01-01", "снимок"},
	}
	for _, c := range cases {
		if got := Frequency(c.points, c.from, c.to).Label; got != c.want {
			t.Errorf("%d %s..%s: %s, ждали %s", c.points, c.from, c.to, got, c.want)
		}
	}
}

func TestHealthFeedsStaleAndMissing(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	feeds := []Feed{
		{ID: "nbu", Title: "ОВДП", Fetch: func(context.Context, Reporter) ([]Security, Result) { return nil, Result{} }},
		{ID: "us", Title: "США", Provides: []string{"a", "b", "c"}},
		{ID: "ez", Title: "Еврозона", Provides: []string{"d"}},
	}
	stats := []SeriesStat{
		{ID: "a", Points: 101, From: "2026-08-01", To: "2026-11-10", UpdatedAt: 100}, // дневной, отстаёт 5 дней: свежий
		{ID: "b", Points: 13, From: "2025-06-01", To: "2026-06-01", UpdatedAt: 200},  // месячный, последняя точка 5 месяцев назад: устарел
		{ID: "x", Points: 2, From: "2026-10-01", To: "2026-11-01", UpdatedAt: 50},    // чужой ряд без источника
		{ID: "d", Points: 1, From: "2026-11-14", To: "2026-11-14", UpdatedAt: now.Add(-48 * time.Hour).Unix()},
	}
	fh, sh := Health(feeds, stats, map[string]string{"ez": "HTTP 429"}, now)
	if len(fh) != 2 || fh[0].ID != "us" || fh[1].ID != "ez" {
		t.Fatalf("источники без Provides пропускаются: %+v", fh)
	}
	us := fh[0]
	if us.Series != 3 || us.Cached != 2 || us.Points != 114 || us.Stale != 1 || us.LastDate != "2026-11-10" || us.UpdatedAt != 200 {
		t.Fatalf("us: %+v", us)
	}
	if fh[1].Error != "HTTP 429" || fh[1].Cached != 1 || fh[1].Stale != 0 {
		t.Fatalf("ez: %+v", fh[1])
	}
	byID := map[string]SeriesHealth{}
	for _, s := range sh {
		byID[s.ID] = s
	}
	if len(sh) != 4 || byID["a"].Stale || byID["a"].Freq != "день" || byID["a"].LagDays != 5 {
		t.Fatalf("a: %+v", byID["a"])
	}
	if !byID["b"].Stale || byID["b"].Freq != "месяц" {
		t.Fatalf("b: %+v", byID["b"])
	}
	if byID["x"].Feed != "" || byID["d"].Freq != "снимок" || byID["d"].LagDays != 2 || byID["d"].Stale {
		t.Fatalf("x/d: %+v %+v", byID["x"], byID["d"])
	}
	for i := 1; i < len(sh); i++ {
		if sh[i-1].ID > sh[i].ID {
			t.Fatalf("ряды по id: %+v", sh)
		}
	}
}

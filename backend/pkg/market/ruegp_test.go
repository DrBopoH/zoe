package market

import (
	"context"
	"strings"
	"testing"
)

func TestRussiaAndEgyptBuildSeries(t *testing.T) {
	c := jpClient(jpServer(t, ""))
	for _, tc := range []struct {
		name, policy, cpi string
		run               func(context.Context, Reporter) ([]SeriesPoint, Result)
	}{
		{SourceRU, SerRUPolicy, SerRUCPI, c.Russia},
		{SourceEG, SerEGPolicy, SerEGCPI, c.Egypt},
	} {
		pts, res := tc.run(context.Background(), nil)
		by := byID(pts)
		if res.Error != "" || res.Source != tc.name || len(by[tc.policy]) != 2 || len(by[tc.cpi]) != 2 {
			t.Fatalf("%s: %+v %v", tc.name, res, by)
		}
	}
}

// Высокая инфляция (Египет, Россия): из индекса годовой рост считается верно и при ста и более процентах.
func TestBISCPIHighInflationFromIndex(t *testing.T) {
	body := "FREQ,REF_AREA,UNIT_MEASURE,TIME_PERIOD,OBS_VALUE\nM,EG,628,2025-09,100\nM,EG,628,2026-09,265\n"
	pts, err := parseBISCPI([]byte(body), "eg_cpi")
	if err != nil || len(pts) != 1 || pts[0].Value < 164.9 || pts[0].Value > 165.1 {
		t.Fatalf("%v %+v", err, pts)
	}
}

// Ворота: недоступный источник (блок, таймаут, 5xx) даёт пустой ряд и ошибку у своей страны; соседи не страдают.
func TestRussiaDownDoesNotAffectNeighbours(t *testing.T) {
	srv := jpServer(t, "M.RU")
	c := jpClient(srv)
	pts, res := c.Russia(context.Background(), nil)
	if len(pts) != 0 || !strings.Contains(res.Error, SerRUPolicy) || !strings.Contains(res.Error, SerRUCPI) {
		t.Fatalf("Россия должна дать ошибку по обоим рядам: %d %q", len(pts), res.Error)
	}
	if pts, res := c.Egypt(context.Background(), nil); res.Error != "" || len(pts) == 0 {
		t.Fatalf("Египет жив: %+v", res)
	}
	if pts, res := c.China(context.Background(), nil); res.Error != "" || len(pts) == 0 {
		t.Fatalf("Китай жив: %+v", res)
	}
}

package market

import (
	"context"
	"strings"
	"testing"
)

func TestCommoditiesBuildsAllSeries(t *testing.T) {
	pts, res := econClient(reerServer(t, "")).Commodities(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourceCommodities || len(by) != len(commoditySeries) || res.Fetched != len(pts) {
		t.Fatalf("%+v %v", res, by)
	}
	for _, id := range commoditySeries {
		if len(by[id]) != 2 {
			t.Errorf("нет ряда %s", id)
		}
	}
	if b := by[SerComBrent]; b[1].Value != 102.25 || b[1].Date != "2026-07-01" {
		t.Fatalf("Brent = DCOILBRENTEU: %+v", b)
	}
}

func TestCommoditiesOneFailureKeepsRest(t *testing.T) {
	pts, res := econClient(reerServer(t, "PCOPPUSDM")).Commodities(context.Background(), nil)
	by := byID(pts)
	if !strings.Contains(res.Error, "PCOPPUSDM") || len(by[SerComCopper]) != 0 || len(by[SerComWTI]) == 0 || len(by) != len(commoditySeries)-1 {
		t.Fatalf("упала только медь: %q %d", res.Error, len(by))
	}
	c := NewClient()
	c.FREDBase = ""
	if pts, res := c.Commodities(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("без FRED опрашивать нечего: %d %q", len(pts), res.Error)
	}
}

func TestMetalsFromNBU(t *testing.T) {
	pts, _ := fxClient(fxServer(t, "")).FXNBU(context.Background(), nil)
	by := byID(pts)
	for _, id := range []string{SerXAUUAH} {
		if len(by[id]) != 1 {
			t.Errorf("золото: %v", by[id])
		}
	}
	// серебро, платина и палладий запрашиваются у НБУ так же, как золото; в фикстуре у них нулевой курс — пропуск, а не сбой
	found := map[string]bool{}
	for _, f := range fxCodes {
		found[f.series] = true
	}
	for _, id := range []string{SerXAGUAH, SerXPTUAH, SerXPDUAH} {
		if !found[id] {
			t.Errorf("нет кода для %s", id)
		}
	}
}

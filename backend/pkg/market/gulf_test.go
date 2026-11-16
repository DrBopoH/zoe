package market

import (
	"context"
	"strings"
	"testing"
)

func gulfClient(t *testing.T, failSub string) *Client {
	t.Helper()
	srv := aubrServer(t, failSub)
	c := aubrClient(srv)
	c.WBBase = srv.URL
	return c
}

func TestGulfPacksBuildSeries(t *testing.T) {
	c := gulfClient(t, "")
	for _, tc := range []struct {
		name string
		get  func(context.Context, Reporter) ([]SeriesPoint, Result)
		src  string
		ids  []string
	}{
		{"sa", c.SaudiArabia, SourceSA, saSeries},
		{"ae", c.UAE, SourceAE, aeSeries},
		{"ir", c.Iran, SourceIR, irSeries},
	} {
		pts, res := tc.get(context.Background(), nil)
		by := byID(pts)
		if res.Error != "" || res.Source != tc.src || res.Fetched != len(pts) {
			t.Fatalf("%s: %+v", tc.name, res)
		}
		for _, id := range tc.ids {
			if len(by[id]) == 0 {
				t.Errorf("%s: нет ряда %s", tc.name, id)
			}
		}
		if len(by) != len(tc.ids) {
			t.Errorf("%s: рядов %d, в Provides %d — список и итог расходятся", tc.name, len(by), len(tc.ids))
		}
	}
	if p := byID(mustPts(c.SaudiArabia(context.Background(), nil)))[SerWM("sa")]; len(p) != 13 || p[12].Value != 130 {
		t.Fatalf("Тадавул: 2600 / 2000 = 130: %+v", p)
	}
	if p := byID(mustPts(c.Iran(context.Background(), nil)))[SerIRCPI]; len(p) != 2 || p[1].Value != 32.5 || p[1].Date != "2024-12-01" {
		t.Fatalf("Иран — годы от старых к новым, null пропущен: %+v", p)
	}
}

func TestGulfPartsFailIndependently(t *testing.T) {
	pts, res := gulfClient(t, "TASI").SaudiArabia(context.Background(), nil)
	by := byID(pts)
	if len(by[SerWM("sa")]) != 0 || len(by[SerSAPolicy]) == 0 || len(by[SerSACPI]) == 0 || !strings.Contains(res.Error, "TASI") {
		t.Fatalf("сбой Yahoo не трогает BIS: %q %v", res.Error, by)
	}
	pts, res = gulfClient(t, "WS_LONG_CPI").UAE(context.Background(), nil)
	by = byID(pts)
	if len(by[SerAECPI]) != 0 || len(by[SerWM("ae")]) == 0 || res.Error == "" {
		t.Fatalf("сбой инфляции не трогает индекс: %q %v", res.Error, by)
	}
	c := NewClient()
	c.WBBase = ""
	if pts, res := c.Iran(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("без Всемирного банка опрашивать нечего: %d %q", len(pts), res.Error)
	}
}

func TestGulfRegistered(t *testing.T) {
	by := map[string]Country{}
	for _, c := range WithSources(Countries(), NewClient().Catalog()) {
		by[c.CC] = c
	}
	for cc, want := range map[string]string{"sa": "sa,fx", "ae": "ae,fx", "ir": "ir"} {
		if got := strings.Join(by[cc].Sources, ","); got != want || !IsSource(cc) {
			t.Errorf("%s: источники %q, ждали %q", cc, got, want)
		}
	}
}

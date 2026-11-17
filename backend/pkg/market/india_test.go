package market

import (
	"context"
	"strings"
	"testing"
)

func TestIndiaBuildsSeries(t *testing.T) {
	c := aubrClient(aubrServer(t, ""))
	pts, res := c.India(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourceIN || res.Fetched != len(pts) || len(by) != len(inSeries) {
		t.Fatalf("%+v %v", res, by)
	}
	if p := by[SerWM("in")]; len(p) != 13 || p[12].Value != 140 {
		t.Fatalf("Sensex: 2800 / 2000 = 140: %+v", p)
	}
	pts, res = aubrClient(aubrServer(t, "BSESN")).India(context.Background(), nil)
	by = byID(pts)
	if len(by[SerWM("in")]) != 0 || len(by[SerINPolicy]) == 0 || !strings.Contains(res.Error, "BSESN") {
		t.Fatalf("сбой Yahoo не трогает BIS: %q %v", res.Error, by)
	}
	cc := map[string]Country{}
	for _, c := range WithSources(Countries(), NewClient().Catalog()) {
		cc[c.CC] = c
	}
	if got := strings.Join(cc["in"].Sources, ","); got != "in,fx" || !IsSource("in") {
		t.Errorf("Индия: свой источник и «Курсы»: %q", got)
	}
}

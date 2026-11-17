package market

import (
	"context"
	"strings"
	"testing"
)

func TestRussiaBuildsSeries(t *testing.T) {
	pts, res := jpClient(jpServer(t, "")).Russia(context.Background(), nil)
	by := byID(pts)
	if res.Error != "" || res.Source != SourceRU || len(by[SerRUPolicy]) != 2 || len(by[SerRUCPI]) != 2 {
		t.Fatalf("%+v %v", res, by)
	}
}

func TestEgyptInflationFromWorldBank(t *testing.T) {
	c := jpClient(jpServer(t, ""))
	c.WBBase = c.BISBase
	pts, res := c.Egypt(context.Background(), nil)
	if res.Error != "" || res.Source != SourceEG || len(pts) != 3 {
		t.Fatalf("%+v %v", res, pts)
	}
	if pts[0].Date != "2021-12-01" || pts[2].Date != "2024-12-01" || pts[2].Value != 28.27 || pts[0].Series != SerEGCPI {
		t.Fatalf("годы от старых к новым, null пропущен, точка на конец года: %+v", pts)
	}
	c.WBBase = ""
	if pts, res := c.Egypt(context.Background(), nil); len(pts) != 0 || res.Error != "" {
		t.Fatalf("без Всемирного банка опрашивать нечего: %d %q", len(pts), res.Error)
	}
}

func TestParseWBErrors(t *testing.T) {
	for _, bad := range []string{"", "<html>blocked</html>", `[{"message":[{"id":"120","value":"Invalid format"}]}]`, `[{"page":1},[]]`, `[{"page":1},[{"date":"2024","value":null}]]`} {
		if _, err := parseWB([]byte(bad), "x"); err == nil {
			t.Errorf("ждали ошибку для %q", bad)
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
	c.WBBase = srv.URL
	if pts, res := c.Egypt(context.Background(), nil); res.Error != "" || len(pts) == 0 {
		t.Fatalf("Египет жив: %+v", res)
	}
	if pts, res := c.China(context.Background(), nil); res.Error != "" || len(pts) == 0 {
		t.Fatalf("Китай жив: %+v", res)
	}
}

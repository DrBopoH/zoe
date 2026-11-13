package db

import (
	"path/filepath"
	"testing"
	"time"

	"economic/pkg/market"
)

func TestMacroSeriesReplaceWholeSeriesAndKeepOthers(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.SaveMacroSeries([]market.SeriesPoint{
		{Series: "policy_rate", Date: "2025-01-01", Value: 13.5},
		{Series: "policy_rate", Date: "2026-01-01", Value: 15.5},
		{Series: "cpi_yoy", Date: "2026-09-01", Value: 6.1},
	}, 10); err != nil {
		t.Fatal(err)
	}
	// новый policy_rate заменяет старый целиком (старая точка 2025 уходит), cpi_yoy не затрагивается
	if err := s.SaveMacroSeries([]market.SeriesPoint{{Series: "policy_rate", Date: "2026-06-01", Value: 15}}, 20); err != nil {
		t.Fatal(err)
	}
	all, upd, err := s.MacroSeries(nil)
	if err != nil {
		t.Fatal(err)
	}
	if g := all["policy_rate"]; len(g) != 1 || g[0].Value != 15 {
		t.Fatalf("policy_rate: %+v", g)
	}
	if g := all["cpi_yoy"]; len(g) != 1 || g[0].Value != 6.1 {
		t.Fatalf("cpi_yoy потерян: %+v", all)
	}
	if upd != 20 {
		t.Fatalf("updated_at %d", upd)
	}
	only, _, _ := s.MacroSeries([]string{"cpi_yoy"})
	if len(only) != 1 || only["cpi_yoy"] == nil {
		t.Fatalf("фильтр по ряду: %+v", only)
	}
	if err := s.SaveMacroSeries(nil, 30); err != nil {
		t.Fatal(err)
	}
}

// Ряды фильтруются в SQL: отдаётся только запрошенное, updated_at — по запрошенным рядам; много идентификаторов — обход лимита переменных.
func TestMacroSeriesFilterAndBigRequest(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.SaveMacroSeries([]market.SeriesPoint{{Series: "a", Date: "2026-01-01", Value: 1}, {Series: "a", Date: "2026-02-01", Value: 2}}, 10)
	s.SaveMacroSeries([]market.SeriesPoint{{Series: "b", Date: "2026-01-01", Value: 3}}, 20)

	got, upd, err := s.MacroSeries([]string{"a", "нет-такого"})
	if err != nil || len(got) != 1 || len(got["a"]) != 2 || got["a"][0].Date != "2026-01-01" || upd != 10 {
		t.Fatalf("%+v %d %v", got, upd, err)
	}
	many := make([]string, maxMacroIDs+50)
	for i := range many {
		many[i] = "x"
	}
	many[0] = "b"
	got, _, err = s.MacroSeries(many)
	if err != nil || len(got) != 1 || len(got["b"]) != 1 {
		t.Fatalf("много идентификаторов: %+v %v", got, err)
	}
}

// Большой ряд пишется одной транзакцией с подготовленным запросом.
func TestSaveMacroSeriesBigSeries(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	pts := make([]market.SeriesPoint, 0, 20000)
	base := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 20000; i++ {
		pts = append(pts, market.SeriesPoint{Series: "big", Date: base.AddDate(0, 0, i).Format("2006-01-02"), Value: float64(i)})
	}
	if err := s.SaveMacroSeries(pts, 1); err != nil {
		t.Fatal(err)
	}
	got, _, _ := s.MacroSeries([]string{"big"})
	if len(got["big"]) != 20000 || got["big"][19999].Value != 19999 {
		t.Fatalf("точек %d", len(got["big"]))
	}
}

// Докачанный хвост заменяет ряд только с первой своей даты; раньшее остаётся, MacroLastDate видит новую границу.
func TestSaveMacroSeriesTailKeepsHistory(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	pt := func(d string, v float64) market.SeriesPoint {
		return market.SeriesPoint{Series: "us_vix", Date: d, Value: v}
	}
	if got := s.MacroLastDate("us_vix"); got != "" {
		t.Fatalf("пустой кеш: %q", got)
	}
	if err := s.SaveMacroSeries([]market.SeriesPoint{pt("2026-10-02", 1), pt("2026-10-09", 2), pt("2026-10-16", 3)}, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMacroSeries([]market.SeriesPoint{pt("2026-10-09", 20), pt("2026-10-23", 40)}, 2, "us_vix"); err != nil {
		t.Fatal(err)
	}
	got, _, _ := s.MacroSeries([]string{"us_vix"})
	want := []market.SeriesPoint{pt("2026-10-02", 1), pt("2026-10-09", 20), pt("2026-10-23", 40)}
	if len(got["us_vix"]) != len(want) {
		t.Fatalf("ряд: %+v", got["us_vix"])
	}
	for i, w := range want {
		if g := got["us_vix"][i]; g.Date != w.Date || g.Value != w.Value {
			t.Fatalf("точка %d: %+v, ждали %+v", i, g, w)
		}
	}
	if d := s.MacroLastDate("us_vix"); d != "2026-10-23" {
		t.Fatalf("MacroLastDate: %q", d)
	}
}

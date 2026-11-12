package db

import (
	"path/filepath"
	"testing"

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

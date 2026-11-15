package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// сервер FRED: у каждого id свой дневной ряд с пропуском «.» и двумя точками одной недели; failID отвечает 502
func fredDailyServer(t *testing.T, failID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == failID {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		// 05.11 — чт, 06.11 — пропуск, 09.11 и 10.11 — одна неделя
		w.Write([]byte("observation_date," + id + "\n2026-11-05,-0.10\n2026-11-06,.\n2026-11-09,0.05\n2026-11-10,0.12\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestUSCurveBuildsWeeklySeriesForEveryTenor(t *testing.T) {
	pts, res := econClient(fredDailyServer(t, "")).USCurve(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	want := []string{SerUST1M, SerUST3M, SerUST6M, SerUST1Y, SerUST2Y, SerUST3Y, SerUST5Y, SerUST7Y, SerUST20Y, SerUST30Y, SerUSSpread3M}
	for _, id := range want {
		s := by[id]
		if len(s) != 2 || s[0].Date != "2026-11-05" || s[0].Value != -0.10 || s[1].Date != "2026-11-10" || s[1].Value != 0.12 {
			t.Fatalf("%s: неделя → последнее значение, отрицательные значения сохраняются: %+v", id, s)
		}
	}
	if len(by) != len(want) {
		t.Fatalf("лишние ряды: %v", by)
	}
	if res.Source != SourceUSCurve || res.Skipped != len(want) {
		t.Fatalf("source=%s skipped=%d (по одной точке «.» на ряд)", res.Source, res.Skipped)
	}
}

func TestUSCurveFailedTenorDoesNotKillOthers(t *testing.T) {
	pts, res := econClient(fredDailyServer(t, "DGS7")).USCurve(context.Background(), nil)
	by := byID(pts)
	if len(by[SerUST7Y]) != 0 || len(by[SerUST2Y]) == 0 || len(by[SerUST30Y]) == 0 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "DGS7") {
		t.Fatalf("error: %q", res.Error)
	}
}

func TestUSRealBuildsRealYieldsAndBreakevens(t *testing.T) {
	pts, res := econClient(fredDailyServer(t, "")).USReal(context.Background(), nil)
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	by := byID(pts)
	for _, id := range []string{SerUSReal5, SerUSReal10, SerUSBE5, SerUSBE10, SerUSFwd5y5y, SerUSTermPrem, SerUSMortgage} {
		if s := by[id]; len(s) != 2 || s[1].Value != 0.12 {
			t.Fatalf("%s: %+v", id, s)
		}
	}
	if res.Source != SourceUSReal {
		t.Fatalf("source=%s", res.Source)
	}
}

func TestUSRealFailedSeriesDoesNotKillOthers(t *testing.T) {
	pts, res := econClient(fredDailyServer(t, "T10YIE")).USReal(context.Background(), nil)
	by := byID(pts)
	if len(by[SerUSBE10]) != 0 || len(by[SerUSReal10]) == 0 || len(by[SerUSBE5]) == 0 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "T10YIE") {
		t.Fatalf("error: %q", res.Error)
	}
}

// у новых источников есть запись в Feeds и они известны маршруту ?source=…
func TestCurveAndRealFeedsAreRegistered(t *testing.T) {
	c := testClient("", "")
	got := map[string]int{}
	for _, f := range c.Feeds() {
		got[f.ID] = len(f.Provides)
	}
	if got[SourceUSCurve] != 11 || got[SourceUSReal] != 7 {
		t.Fatalf("Provides: %v", got)
	}
	if !IsSource(SourceUSCurve) || !IsSource(SourceUSReal) {
		t.Fatal("IsSource")
	}
}

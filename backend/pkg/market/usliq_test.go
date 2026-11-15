package market

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pt(date string, v float64) SeriesPoint { return SeriesPoint{Date: date, Value: v} }

func TestScalePointsDividesAndRenames(t *testing.T) {
	got := scalePoints([]SeriesPoint{pt("2026-11-11", 6758867)}, "us_fedbs", 1000)
	if len(got) != 1 || got[0].Series != "us_fedbs" || !near(got[0].Value, 6758.867) {
		t.Fatalf("%+v", got)
	}
}

func TestNetLiquidityIsBalanceMinusTreasuryMinusRepo(t *testing.T) {
	fedbs := []SeriesPoint{pt("2026-11-04", 6700), pt("2026-11-11", 6750)}
	tga := []SeriesPoint{pt("2026-11-04", 800), pt("2026-11-11", 900)}
	rrp := []SeriesPoint{pt("2026-11-03", 10), pt("2026-11-10", 20)} // РЕПО идёт по другим дням: берётся последнее не позже даты баланса
	got := netLiquidity(fedbs, tga, rrp)
	if len(got) != 2 || got[0].Series != SerUSNetLiq || got[0].Value != 6700-800-10 || got[1].Value != 6750-900-20 {
		t.Fatalf("%+v", got)
	}
}

func TestNetLiquidityIgnoresStaleOrMissingParts(t *testing.T) {
	fedbs := []SeriesPoint{pt("2026-11-11", 6750)}
	if got := netLiquidity(fedbs, nil, []SeriesPoint{pt("2026-11-10", 1)}); len(got) != 0 {
		t.Fatalf("нет счёта Казначейства — нет и ликвидности: %+v", got)
	}
	stale := []SeriesPoint{pt("2026-10-01", 900)}
	if got := netLiquidity(fedbs, stale, []SeriesPoint{pt("2026-11-10", 1)}); len(got) != 0 {
		t.Fatalf("счёт старше 10 дней не берётся: %+v", got)
	}
}

// сервер FRED: баланс и счёт в миллионах (6 750 000 и 900 000), РЕПО в миллиардах (20), M2 — индекс 1 % в месяц
func fredLiquidityServer(t *testing.T, failID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == failID {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		switch id {
		case "WALCL":
			w.Write([]byte("observation_date,WALCL\n2026-11-04,6700000\n2026-11-11,6750000\n"))
		case "WTREGEN":
			w.Write([]byte("observation_date,WTREGEN\n2026-11-04,800000\n2026-11-11,900000\n"))
		case "RRPONTSYD":
			w.Write([]byte("observation_date,RRPONTSYD\n2026-11-03,10\n2026-11-10,20\n2026-11-12,.\n"))
		case "M2SL":
			var b strings.Builder
			b.WriteString("observation_date,M2SL\n")
			for i := 0; i < 14; i++ {
				b.WriteString([]string{"2025-01-01", "2025-02-01", "2025-03-01", "2025-04-01", "2025-05-01", "2025-06-01", "2025-07-01", "2025-08-01", "2025-09-01", "2025-10-01", "2025-11-01", "2025-12-01", "2026-01-01", "2026-02-01"}[i] + ",")
				b.WriteString(fmt.Sprintf("%.6f", 20000*math.Pow(1.005, float64(i))) + "\n")
			}
			w.Write([]byte(b.String()))
		default:
			w.Write([]byte("observation_date," + id + "\n2026-11-09,3.9\n"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestUSLiquidityConvertsUnitsAndBuildsNetLiquidity(t *testing.T) {
	pts, res := econClient(fredLiquidityServer(t, "")).USLiquidity(context.Background(), nil)
	if res.Source != SourceUSLiq || res.Error != "" {
		t.Fatalf("res: %+v", res)
	}
	by := byID(pts)
	if s := by[SerUSFedBS]; len(s) != 2 || !near(s[1].Value, 6750) {
		t.Fatalf("баланс в млрд: %+v", s)
	}
	if s := by[SerUSTGA]; len(s) != 2 || !near(s[1].Value, 900) {
		t.Fatalf("счёт в млрд: %+v", s)
	}
	if s := by[SerUSNetLiq]; len(s) != 2 || !near(s[1].Value, 6750-900-20) {
		t.Fatalf("чистая ликвидность: %+v", s)
	}
	if s := by[SerUSRRP]; len(s) != 2 || s[1].Value != 20 {
		t.Fatalf("РЕПО по неделям, пропуск «.» отброшен: %+v", s)
	}
	if s := by[SerUSM2YoY]; len(s) != 2 || !near(s[0].Value, (math.Pow(1.005, 12)-1)*100) {
		t.Fatalf("M2 г/г: %+v", s)
	}
	if len(by[SerUSM2]) != 14 || len(by[SerUSSOFR]) != 1 {
		t.Fatalf("M2 %d, SOFR %d", len(by[SerUSM2]), len(by[SerUSSOFR]))
	}
}

func TestUSLiquidityFailedSeriesDoesNotKillOthers(t *testing.T) {
	pts, res := econClient(fredLiquidityServer(t, "WTREGEN")).USLiquidity(context.Background(), nil)
	by := byID(pts)
	if len(by[SerUSTGA]) != 0 || len(by[SerUSNetLiq]) != 0 || len(by[SerUSFedBS]) != 2 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "WTREGEN") {
		t.Fatalf("error: %q", res.Error)
	}
}

func TestUSCreditBuildsSeriesAndIsolatesFailures(t *testing.T) {
	pts, res := econClient(fredMonthlyServer(t, nil, "NFCI")).USCredit(context.Background(), nil)
	by := byID(pts)
	if len(by[SerUSHY]) == 0 || len(by[SerUSIG]) == 0 || len(by[SerUSUSD]) == 0 || len(by[SerUSDebtGDP]) != 15 || len(by[SerUSNFCI]) != 0 {
		t.Fatalf("series: %v", by)
	}
	if res.Source != SourceUSCredit || !strings.Contains(res.Error, "NFCI") {
		t.Fatalf("res: %+v", res)
	}
}

func TestLiquidityAndCreditFeedsAreRegistered(t *testing.T) {
	got := map[string]int{}
	for _, f := range testClient("", "").Feeds() {
		got[f.ID] = len(f.Provides)
	}
	if got[SourceUSLiq] != 7 || got[SourceUSCredit] != 5 || !IsSource(SourceUSLiq) || !IsSource(SourceUSCredit) {
		t.Fatalf("Provides: %v", got)
	}
}

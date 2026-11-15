package market

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// сервер FRED: индексы растут на 1 % в месяц (15 месяцев с 2025-01), остальные ряды — линейные уровни 100, 110, 120…
func fredMonthlyServer(t *testing.T, indexIDs []string, failID string) *httptest.Server {
	t.Helper()
	isIndex := map[string]bool{}
	for _, id := range indexIDs {
		isIndex[id] = true
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == failID {
			http.Error(w, "x", http.StatusBadGateway)
			return
		}
		var b strings.Builder
		b.WriteString("observation_date," + id + "\n")
		for i := 0; i < 15; i++ {
			d := time.Date(2025, time.Month(1+i), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			v := 100 + 10*float64(i)
			if isIndex[id] {
				v = 100 * math.Pow(1.01, float64(i))
			}
			fmt.Fprintf(&b, "%s,%.6f\n", d, v)
		}
		w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestIndexChangesYoYAndMoM(t *testing.T) {
	obs := []fredObs{{"2025-01-01", 100}, {"2025-02-01", 101}, {"2026-01-01", 110}, {"2026-02-01", 111}}
	yoy, mom := indexChanges(obs, "y", "m")
	if len(yoy) != 2 || yoy[0].Date != "2026-01-01" || !near(yoy[0].Value, 10) || !near(yoy[1].Value, (111.0/101-1)*100) {
		t.Fatalf("yoy: %+v", yoy)
	}
	if len(mom) != 2 || !near(mom[0].Value, 1) || !near(mom[1].Value, (111.0/110-1)*100) {
		t.Fatalf("mom: %+v", mom)
	}
	if y, m := indexChanges(obs, "y", ""); len(y) != 2 || m != nil {
		t.Fatalf("пустой id = ряд не считается: %+v %+v", y, m)
	}
}

func TestIndexChangesQuarterlyHasYoYButNoMoM(t *testing.T) {
	obs := []fredObs{{"2025-01-01", 100}, {"2025-04-01", 101}, {"2025-07-01", 102}, {"2025-10-01", 103}, {"2026-01-01", 104}}
	yoy, mom := indexChanges(obs, "eci", "m")
	if len(yoy) != 1 || yoy[0].Date != "2026-01-01" || !near(yoy[0].Value, 4) {
		t.Fatalf("yoy: %+v", yoy)
	}
	if len(mom) != 0 {
		t.Fatalf("у квартального ряда месячного изменения нет: %+v", mom)
	}
}

func TestLevelChangesAreDifferencesInOrder(t *testing.T) {
	got := levelChanges([]fredObs{{"2025-03-01", 130}, {"2025-01-01", 100}, {"2025-02-01", 115}}, "nfp")
	if len(got) != 2 || got[0].Date != "2025-02-01" || got[0].Value != 15 || got[1].Date != "2025-03-01" || got[1].Value != 15 {
		t.Fatalf("%+v", got)
	}
	if levelChanges([]fredObs{{"2025-01-01", 1}}, "nfp") != nil {
		t.Fatal("одна точка — разности нет")
	}
}

func TestUSPricesBuildsYoYForEveryIndex(t *testing.T) {
	srv := fredMonthlyServer(t, []string{"CPILFESL", "PCEPI", "PCEPILFE", "PPIACO", "CUSR0000SAH1"}, "")
	pts, res := econClient(srv).USPrices(context.Background(), nil)
	if res.Error != "" || res.Source != SourceUSPrices {
		t.Fatalf("res: %+v", res)
	}
	by := byID(pts)
	want := (math.Pow(1.01, 12) - 1) * 100
	for _, id := range []string{SerUSCoreCPIYoY, SerUSPCEYoY, SerUSCorePCEYoY, SerUSPPIYoY, SerUSShelterYoY} {
		s := by[id]
		if len(s) != 3 || s[0].Date != "2026-01-01" || !near(s[0].Value, want) {
			t.Fatalf("%s: %+v", id, s)
		}
	}
	if m := by[SerUSCoreCPIMoM]; len(m) != 14 || !near(m[0].Value, 1) {
		t.Fatalf("core mom: %+v", m)
	}
}

func TestUSPricesFailedSeriesDoesNotKillOthers(t *testing.T) {
	srv := fredMonthlyServer(t, []string{"CPILFESL", "PCEPI", "PCEPILFE", "PPIACO", "CUSR0000SAH1"}, "PPIACO")
	pts, res := econClient(srv).USPrices(context.Background(), nil)
	by := byID(pts)
	if len(by[SerUSPPIYoY]) != 0 || len(by[SerUSPCEYoY]) == 0 || len(by[SerUSCoreCPIYoY]) == 0 {
		t.Fatalf("series: %v", by)
	}
	if !strings.Contains(res.Error, "PPIACO") {
		t.Fatalf("error: %q", res.Error)
	}
}

func TestUSJobsBuildsPayrollChangeAndWages(t *testing.T) {
	srv := fredMonthlyServer(t, []string{"CES0500000003", "ECIALLCIV"}, "")
	pts, res := econClient(srv).USJobs(context.Background(), nil)
	if res.Error != "" || res.Source != SourceUSJobs {
		t.Fatalf("res: %+v", res)
	}
	by := byID(pts)
	if s := by[SerUSNFP]; len(s) != 14 || s[0].Value != 10 || s[13].Value != 10 {
		t.Fatalf("nfp: %+v", s)
	}
	for _, id := range []string{SerUSClaims, SerUSJOLTS, SerUSU6} {
		if s := by[id]; len(s) != 15 || s[0].Value != 100 {
			t.Fatalf("%s: %+v", id, s)
		}
	}
	if s := by[SerUSWages]; len(s) != 3 || !near(s[0].Value, (math.Pow(1.01, 12)-1)*100) {
		t.Fatalf("wages: %+v", s)
	}
	if s := by[SerUSECIYoY]; len(s) != 3 {
		t.Fatalf("eci на месячных данных тоже считается: %+v", s)
	}
}

func TestUSActivityBuildsYoYAndHousingLevel(t *testing.T) {
	srv := fredMonthlyServer(t, []string{"RSAFS", "INDPRO", "CSUSHPINSA"}, "HOUST")
	pts, res := econClient(srv).USActivity(context.Background(), nil)
	by := byID(pts)
	for _, id := range []string{SerUSRetailYoY, SerUSIndproYoY, SerUSHomePriceYoY} {
		if len(by[id]) != 3 {
			t.Fatalf("%s: %+v", id, by[id])
		}
	}
	if len(by[SerUSHousing]) != 0 || !strings.Contains(res.Error, "HOUST") {
		t.Fatalf("упавший ряд не мешает остальным, ошибка названа: %q", res.Error)
	}
}

func TestPricesJobsActivityFeedsAreRegistered(t *testing.T) {
	got := map[string]int{}
	for _, f := range testClient("", "").Feeds() {
		got[f.ID] = len(f.Provides)
	}
	if got[SourceUSPrices] != 6 || got[SourceUSJobs] != 6 || got[SourceUSAct] != 4 {
		t.Fatalf("Provides: %v", got)
	}
	for _, s := range []string{SourceUSPrices, SourceUSJobs, SourceUSAct} {
		if !IsSource(s) {
			t.Fatal("IsSource " + s)
		}
	}
}

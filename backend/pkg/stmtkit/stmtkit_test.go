package stmtkit

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"testing"
)

// Эталон — прежние копии из парсеров (raif, privat, novapay): ID обязаны совпасть побайтово.
func oldRaif(account string, ts, op, fee int64, desc string, seen map[string]int) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d|%d|%s", account, ts, op, fee, desc)))
	base := hex.EncodeToString(sum[:])[:20]
	n := seen[base]
	seen[base] = n + 1
	if n == 0 {
		return "csv_raif_" + base
	}
	return fmt.Sprintf("csv_raif_%s_%d", base, n)
}

func oldNova(account string, ts, cents int64, desc string, seen map[string]int) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d|%s", account, ts, cents, desc)))
	base := hex.EncodeToString(sum[:])[:20]
	n := seen[base]
	seen[base]++
	if n == 0 {
		return "csv_novapay_" + base
	}
	return fmt.Sprintf("csv_novapay_%s_%d", base, n)
}

func TestStableIDMatchesOldImplementations(t *testing.T) {
	a, b := map[string]int{}, map[string]int{}
	c, d := map[string]int{}, map[string]int{}
	for i := 0; i < 3; i++ { // три одинаковые операции: суффиксы _1, _2
		want := oldRaif("UA1", 1700000000, -1234, 0, "кава", a)
		got := StableID("csv_raif_", fmt.Sprintf("%s|%d|%d|%d|%s", "UA1", 1700000000, -1234, 0, "кава"), b)
		if want != got {
			t.Fatalf("raif %d: %s != %s", i, want, got)
		}
		wantN := oldNova("UA2", 1700000001, 500, "переказ", c)
		gotN := StableID("csv_novapay_", fmt.Sprintf("%s|%d|%d|%s", "UA2", 1700000001, 500, "переказ"), d)
		if wantN != gotN {
			t.Fatalf("novapay %d: %s != %s", i, wantN, gotN)
		}
	}
}

func TestCentsAndRound2(t *testing.T) {
	cases := map[float64]int64{0: 0, 1.005: 100, 12.345: 1235, -3.335: -334, 0.29: 29}
	for v, want := range cases {
		if got := Cents(v); got != want && math.Abs(float64(got-want)) > 1 {
			t.Errorf("Cents(%v) = %d, want %d", v, got, want)
		}
	}
	if Round2(2.675) != math.Round(2.675*100)/100 {
		t.Error("Round2 расходится с эталонной формулой")
	}
}

func TestKyivIsLoaded(t *testing.T) {
	if Kyiv.String() != "Europe/Kyiv" {
		t.Fatalf("часовой пояс %s", Kyiv)
	}
}

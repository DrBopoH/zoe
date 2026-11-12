// Package stmtkit — общие мелочи парсеров выписок (raif, privat, novapay, icu): ID транзакций, копейки, киевское время.
// Раньше каждый парсер держал свою копию; ID должны оставаться неизменными, иначе повторный импорт создаст дубли.
package stmtkit

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"time"
	_ "time/tzdata" // Europe/Kyiv и на системах без базы часовых поясов
)

// Kyiv — время в выписках киевское, не зависим от часового пояса сервера.
var Kyiv = func() *time.Location {
	if l, err := time.LoadLocation("Europe/Kyiv"); err == nil {
		return l
	}
	return time.Local
}()

// Cents — деньги в копейках (округление к ближайшей, .5 от нуля).
func Cents(v float64) int64 { return int64(math.Round(v * 100)) }

// Round2 — округление до двух знаков.
func Round2(v float64) float64 { return math.Round(v*100) / 100 }

// StableID — стабильный ID операции: повторный импорт той же или перекрывающейся выписки даёт те же ID
// (номер строки файла в ID не входит — сдвиг строк между выгрузками создавал бы дубли).
// key — поля операции, собранные вызывающим («счёт|время|сумма|…»); одинаковые операции в одну минуту
// различаются порядковым суффиксом, для чего нужен общий на выписку seen.
func StableID(prefix, key string, seen map[string]int) string {
	sum := sha1.Sum([]byte(key))
	base := hex.EncodeToString(sum[:])[:20]
	n := seen[base]
	seen[base] = n + 1
	if n == 0 {
		return prefix + base
	}
	return fmt.Sprintf("%s%s_%d", prefix, base, n)
}

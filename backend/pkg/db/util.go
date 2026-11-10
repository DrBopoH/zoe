package db

import (
	"regexp"
	"strings"
)

var (
	reNonAlnum     = regexp.MustCompile(`[^a-zA-Z0-9]`)
	reIBAN         = regexp.MustCompile(`^[A-Z]{2}\d{2}`)
	rePersonPrefix = regexp.MustCompile(`(?i)^(від:|переказ\s+від:|переказ\s+на:|переказ:|для:|з\s+карти|на\s+карту)`)
	reMaskOnly     = regexp.MustCompile(`^[\d\*\s]+$`)
	reParen        = regexp.MustCompile(`\s*\([^)]*\)`)
)

// Эти функции — точные копии frontend (formatters.js / personLayout.js).

func normID(s string) string {
	return strings.ToUpper(reNonAlnum.ReplaceAllString(s, ""))
}

func isIBANLike(s string) bool { return reIBAN.MatchString(normID(s)) }

// idKey — устойчивый ключ идентификатора: IBAN целиком, PAN — первые 4 + последние 4
// (маски 487407******8644 и 4874****8644 дают один ключ).
func idKey(s string) string {
	n := normID(s)
	if reIBAN.MatchString(n) {
		return n
	}
	if len(n) >= 8 {
		return n[:4] + n[len(n)-4:]
	}
	return n
}

func aliasKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func cleanAccountName(name string) string {
	return strings.TrimSpace(reParen.ReplaceAllString(name, ""))
}

func cleanPersonName(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimSpace(rePersonPrefix.ReplaceAllString(s, ""))
	if reMaskOnly.MatchString(s) {
		return ""
	}
	return s
}

func splitPans(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// mergePans: первый список приоритетнее; дубли (по idKey) отбрасываются.
func mergePans(lists ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, l := range lists {
		for _, p := range l {
			p = strings.TrimSpace(p)
			k := idKey(p)
			if p == "" || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, p)
		}
	}
	return out
}

// isPanLike: номер карты (полный или с маской), но не IBAN.
func isPanLike(s string) bool {
	n := normID(s)
	return n != "" && !reIBAN.MatchString(n) && len(n) >= 8 && strings.Trim(n, "0123456789") == ""
}

// looseKey — «слабый» ключ карты: первые 6 цифр (BIN) + последние 2.
// Нужен потому, что ПриватБанк в переводах показывает получателя как 414949****72,
// а свои и чужие карты — как 414949******7277: по idKey (4+4) они не сойдутся, по BIN+2 — сойдутся.
// Пустая строка — если по номеру ключ не построить (мало открытых цифр).
func looseKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || reIBAN.MatchString(normID(s)) {
		return ""
	}
	i := strings.IndexAny(s, "*•xX")
	if i < 0 { // полный номер
		n := reNonAlnum.ReplaceAllString(s, "")
		if len(n) >= 12 && strings.Trim(n, "0123456789") == "" {
			return n[:6] + n[len(n)-2:]
		}
		return ""
	}
	lead := digitsOnly(s[:i])
	tail := digitsOnly(s[i:])
	if len(lead) < 6 || len(tail) < 2 {
		return ""
	}
	return lead[:6] + tail[len(tail)-2:]
}

// reliableKey — idKey, но "" для масок с <4 открытыми цифрами в конце (414949****72):
// их «4+4» ключ — мусор и мог бы случайно совпасть с чужой картой.
func reliableKey(s string) string {
	if i := strings.IndexAny(s, "*•xX"); i >= 0 && len(digitsOnly(s[i:])) < 4 {
		return ""
	}
	return idKey(s)
}

// isPartialPAN — карта с <4 открытыми цифрами в конце (414949****72): точно сравнить нельзя, только по BIN+2.
func isPartialPAN(s string) bool { return isPanLike(s) && reliableKey(s) == "" && looseKey(s) != "" }

// sameID — один и тот же идентификатор В ПРЕДЕЛАХ ОДНОГО АЛИАСА: совпал idKey, либо одна из карт
// «частичная» и у них равен BIN+2 (414949****72 ≡ 414949******0172). Между разными алиасами так не склеиваем.
func sameID(a, b string) bool {
	if idKey(a) == idKey(b) {
		return true
	}
	la, lb := looseKey(a), looseKey(b)
	return la != "" && la == lb && (isPartialPAN(a) || isPartialPAN(b))
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isMaskedID: номер карты со скрытой серединой (414951******5262)
func isMaskedID(s string) bool { return strings.ContainsAny(s, "*•") }

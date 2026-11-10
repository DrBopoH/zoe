package db

import "testing"

// Выписка Райфа допускает «Від Імʼя» без двокрапки.
func TestCleanPersonNamePrefix(t *testing.T) {
	cases := map[string]string{
		"Від Резніченко Наталя": "Резніченко Наталя",
		"Від: Іван":             "Іван",
		"переказ від Ірина":     "Ірина",
		"Віденський Іван":       "Віденський Іван",
	}
	for in, want := range cases {
		if got := cleanPersonName(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

func TestPseudoCounterName(t *testing.T) {
	desc := "Перекази, УКРАЇНА, термінал I0110SW5, адреса терміналу -, документ № P24A59, отримувач -, 487407****44, банк отримувача JSC UNIVERSAL BANK, код ІД НБУ 322001"
	cases := []struct{ c, d, want string }{
		{"487407****44", desc, "487407****44 · Monobank"},
		{"414949****72", "", "414949****72 · PrivatBank"},
		{"999999****11", "отримувач -, 999999****11, банк отримувача ACME BANK, код", "999999****11 · ACME BANK"},
		{"999999****11", "", "999999****11"},
		{"UA213220010000000000000006001", desc, ""},
	}
	for _, c := range cases {
		if got := pseudoCounterName(c.c, c.d); got != c.want {
			t.Errorf("%q: got %q want %q", c.c, got, c.want)
		}
	}
}

package market

import (
	"strings"
	"testing"
)

func TestWithSourcesFollowsCatalog(t *testing.T) {
	cat := NewClient().Catalog()
	by := map[string]Country{}
	for _, c := range WithSources(Countries(), cat) {
		by[c.CC] = c
	}
	if got := strings.Join(by["tw"].Sources, ","); got != "tw,fx" {
		t.Errorf("tw: индекс — свой источник, реальный курс — «Курсы»: %s", got)
	}
	if got := strings.Join(by["ua"].Sources, ","); got != "macro" {
		t.Errorf("ua: всё из одного источника, без повторов: %s", got)
	}
	// свой источник первым, у каждой страны есть он сам, повторов нет
	for _, c := range WithSources(Countries(), cat) {
		seen := map[string]bool{}
		for _, s := range c.Sources {
			if seen[s] || !IsSource(s) {
				t.Errorf("%s: повтор или неизвестный источник %q в %v", c.CC, s, c.Sources)
			}
			seen[s] = true
		}
		if len(c.Sources) == 0 || c.Sources[0] != c.Source {
			t.Errorf("%s: свой источник должен идти первым: %v", c.CC, c.Sources)
		}
	}
	// без каталога (клиента нет) остаётся хотя бы свой источник
	if got := WithSources(Countries(), nil)[0].Sources; len(got) != 1 || got[0] != SourceUS {
		t.Errorf("без каталога: %v", got)
	}
}

func TestSplitSources(t *testing.T) {
	if got := SplitSources(" Kr, FX ,,"); len(got) != 2 || got[0] != "kr" || got[1] != "fx" {
		t.Errorf("%v", got)
	}
	if SplitSources("") != nil || !IsSources("") || !IsSources("kr,fx") || IsSources("kr,zz") {
		t.Error("пусто — «все»; неизвестный источник в списке — отказ")
	}
}

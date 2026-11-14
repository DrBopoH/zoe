package market

import "testing"

// Каталог — единственный перечень рядов: у каждого числового источника он непустой, id не повторяются ни внутри, ни между источниками,
// а у источников облигаций рядов нет.
func TestCatalogIsConsistent(t *testing.T) {
	c := testClient("", "")
	seen := map[string]string{}
	for _, f := range c.Feeds() {
		if (f.Series != nil) != (len(f.Provides) > 0) {
			t.Errorf("%s: Provides должен быть у числового источника и только у него", f.ID)
		}
		for _, id := range f.Provides {
			if prev, dup := seen[id]; dup {
				t.Errorf("ряд %s заявлен и в %s, и в %s", id, prev, f.ID)
			}
			seen[id] = f.ID
		}
	}
	cat := c.Catalog()
	if len(cat) != 6 || len(cat["fx"]) != 3 || cat["fx"][0] != SerUSDUAH || len(cat["macro"]) != 8 {
		t.Fatalf("каталог: %v", cat)
	}
	if _, ok := cat["nbu"]; ok {
		t.Error("у ОВДП нет числовых рядов")
	}
}

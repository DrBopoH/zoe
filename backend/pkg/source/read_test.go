package source

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gzip-«бомба»: маленький файл, огромный после распаковки — читать нельзя, память не должна расти без предела.
func TestReadRejectsGzipBomb(t *testing.T) {
	old := maxUnpacked
	maxUnpacked = 1 << 20
	t.Cleanup(func() { maxUnpacked = old })
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	chunk := make([]byte, 1<<20)
	for i := 0; i <= maxUnpacked>>20; i++ { // чуть больше предела; нули сжимаются в крошку
		if _, err := zw.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "bomb.zoe")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(p); err == nil || !strings.Contains(err.Error(), "больше") {
		t.Fatalf("ждали отказ по размеру, получили %v", err)
	}
}

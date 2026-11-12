package api

import (
	"os"
	"strings"
	"testing"
)

// Тесты не должны трогать ничего за пределами временной папки. Импорт CSV кладёт файлы-источники в
// ~/.local/share/economic/sources, а при «Загрузить» они попадают в настоящую базу — именно так тестовая
// выписка (UA58…1234567, карта 4149****7777) однажды просочилась в реальный кеш. Поэтому для всего пакета
// папка источников и «домашняя» папка подменяются.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "economic-api-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("ECONOMIC_SOURCES_DIR", tmp+"/sources")
	os.Setenv("XDG_DATA_HOME", tmp+"/xdg")
	os.Setenv("HOME", tmp+"/home")
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// Страховка: если кто-то уберёт подмену из TestMain — тест упадёт, а не молча начнёт писать в настоящий $HOME.
func TestSourcesDirIsIsolated(t *testing.T) {
	if d := sourcesDir(); !strings.HasPrefix(d, os.TempDir()) {
		t.Fatalf("папка источников вне временной: %s", d)
	}
}

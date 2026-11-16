package market

import (
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	DefaultRetryWait = time.Millisecond // повторы запросов не должны растягивать тесты на секунды
	os.Exit(m.Run())
}

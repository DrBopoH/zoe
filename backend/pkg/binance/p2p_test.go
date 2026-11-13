package binance

import (
	"math"
	"os"
	"strings"
	"testing"
)

const p2pName = "Binance-Історія-C2C-ордерів-202611131224(UTC+2)-part1-of1.csv"

func p2pFixture(t *testing.T) []P2POrder {
	t.Helper()
	raw, err := os.ReadFile("testdata/p2p.csv")
	if err != nil {
		t.Fatal(err)
	}
	if !IsP2P(raw) || IsStatement(raw) {
		t.Fatal("история ордеров — не выписка транзакций, и наоборот")
	}
	out, err := ParseP2P(strings.NewReader(string(raw)), StatementZone(p2pName))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestP2PKeepsEveryOrder(t *testing.T) {
	o := p2pFixture(t)
	if len(o) != 3 {
		t.Fatalf("все ордера, в том числе отменённый: %d", len(o))
	}
	a := o[0]
	// 07:56:07 по UTC+2 — 05:56:07 UTC
	if a.ID != "22889931328257875968" || a.Side != "Buy" || a.Asset != "USDT" || a.Fiat != "UAH" || a.Total != 1125 || a.Price != 44.79 ||
		a.Qty != 25.11 || a.Fee != 0.05 || a.Status != "Completed" || a.T != 1779170167000 || !a.Done() {
		t.Fatalf("первый ордер: %+v", a)
	}
	if o[2].Done() || o[2].Status != "Cancelled" {
		t.Fatalf("отменённый ордер сохраняется со статусом: %+v", o[2])
	}
}

// Покупка в истории ордеров и P2P-строка выписки — одна и та же операция: на кошелёк приходит количество минус комиссия,
// а в журнале зачисление лежит через несколько минут после создания ордера.
func TestP2PCreditMatchesLedger(t *testing.T) {
	st := fixture(t)
	for _, o := range p2pFixture(t)[:2] {
		found := false
		for _, r := range st.Rows {
			if Kind(r) == KindP2P && r.Asset == o.Asset && math.Abs(r.Delta-o.Credit()) < 0.0001 && r.T >= o.T && r.T-o.T < 30*60*1000 {
				found = true
			}
		}
		if !found {
			t.Fatalf("для ордера %s (%.2f %s) нет зачисления в журнале", o.ID, o.Credit(), o.Asset)
		}
	}
}

func TestP2PRejectsOtherFiles(t *testing.T) {
	if _, err := ParseP2P(strings.NewReader("Ідентифікатор користувача,Час\n1,2"), nil); err == nil {
		t.Fatal("выписка транзакций не история ордеров")
	}
	bad := "Номер ордера,Тип\n1,Buy,USDT,UAH,x,1,1,,,0,A,Completed,2026-05-19 07:56:07\n"
	if _, err := ParseP2P(strings.NewReader(bad), nil); err == nil {
		t.Fatal("нечисловая сумма должна быть ошибкой, а не нулём")
	}
}

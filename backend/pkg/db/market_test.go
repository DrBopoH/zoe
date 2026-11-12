package db

import (
	"path/filepath"
	"testing"
	"time"

	"economic/pkg/market"
)

func TestMarketBondsMergeKeepsKnownAndFeedsICUTerms(t *testing.T) {
	d := t.TempDir()
	s, err := NewStorage(filepath.Join(d, "c.db"), filepath.Join(d, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// реестр (условия), потом аукцион (доходность без ставки) — ставка и дата не должны потеряться
	if err := s.SaveMarketBonds([]market.Security{
		{Source: "nbu", ID: "ua4000239081", Name: "ОВДП 2030", Currency: "UAH", Maturity: "2030-02-20", CouponRate: 16.44, Nominal: 1000},
		{Source: "nbu", ID: "UA4000000777", Name: "USD", Currency: "USD", Maturity: "2030-01-01", CouponRate: 4, Nominal: 1000},
		{Source: "ust", ID: "912810XX1", Name: "30-Year Bond", Currency: "USD", Maturity: "2056-11-15", CouponRate: 4.625, Nominal: 100},
	}, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMarketBonds([]market.Security{{Source: "nbu", ID: "UA4000239081", Yield: 16.1, LastAuction: "2026-11-12"}}, 2); err != nil {
		t.Fatal(err)
	}
	nbu, err := s.MarketBonds("nbu")
	if err != nil || len(nbu) != 2 {
		t.Fatalf("nbu: %v %+v", err, nbu)
	}
	var a market.Security
	for _, b := range nbu {
		if b.ID == "UA4000239081" {
			a = b
		}
	}
	if a.CouponRate != 16.44 || a.Maturity != "2030-02-20" || a.Yield != 16.1 || a.LastAuction != "2026-11-12" || a.Name != "ОВДП 2030" {
		t.Fatalf("пустое не должно затирать известное: %+v", a)
	}
	if all, _ := s.MarketBonds(""); len(all) != 3 {
		t.Fatalf("все источники: %d", len(all))
	}

	// условия для расчёта: скачанная гривневая бумага подходит, USD и US Treasury — нет
	terms, err := s.bondTerms()
	if err != nil || len(terms) != 1 || terms[0].ISIN != "UA4000239081" || terms[0].CouponRate != 16.44 {
		t.Fatalf("bondTerms: %v %+v", err, terms)
	}
	// своя запись главнее скачанной
	if _, err := s.SaveBond(Bond{ISIN: "UA4000239081", Maturity: "2030-02-20", CouponRate: 10, Nominal: 1000}); err != nil {
		t.Fatal(err)
	}
	terms, _ = s.bondTerms()
	if len(terms) != 1 || terms[0].CouponRate != 10 {
		t.Fatalf("ручной справочник должен перекрывать скачанный: %+v", terms)
	}
	// справочник для редактора (ListBonds) скачанным не засоряется
	if own, _ := s.ListBonds(); len(own) != 1 {
		t.Fatalf("ListBonds только свои: %+v", own)
	}
	// ICU считает купоны по скачанным условиям, если своих нет
	if err := s.DeleteBond("UA4000239081"); err != nil {
		t.Fatal(err)
	}
	terms, _ = s.bondTerms()
	rows := []LedgerRow{
		lrow(2026, 11, 5, KindDeposit, 3200, "", 0),
		lrow(2026, 11, 5, KindBuy, -3070, "UA4000239081", 3),
	}
	st := ComputeState(rows, terms, time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local))
	if len(st.Positions) != 1 || st.Positions[0].NoTerms || st.Positions[0].Accrued != 34.33 {
		t.Fatalf("скачанные условия должны дать НКД как у ручных: %+v", st.Positions)
	}

	if err := s.WipeCache(); err != nil {
		t.Fatal(err)
	}
	if all, _ := s.MarketBonds(""); len(all) != 0 {
		t.Fatalf("вайп кеша должен стирать и рыночные данные: %d", len(all))
	}
}

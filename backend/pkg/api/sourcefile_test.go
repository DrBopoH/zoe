package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"economic/pkg/db"
	"economic/pkg/mono"
	"economic/pkg/source"
)

// Брокер и Binance проходят тот же круг, что банки: файл .zoe → сброс кеша → «Загрузить» из источников.
func TestZoeRoundTripBrokerAndBinance(t *testing.T) {
	t.Setenv("ECONOMIC_SOURCES_DIR", t.TempDir())
	s := newICUServer(t)

	icuF := icuFile("ОВГЗ Тест.pdf", fixtureReport(t))
	now := time.Now().UTC().Truncate(time.Minute)
	bnF, err := parseSource(stmtName, []byte(stmtFor(now.Add(-3*time.Hour), now.Add(-2*time.Hour))))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []*source.File{icuF, bnF} {
		if _, added, err := store().Put(f); err != nil || !added {
			t.Fatalf("put %s: %v %v", f.Kind, added, err)
		}
		if _, err := s.applySource(f); err != nil {
			t.Fatal(err)
		}
	}
	if icuF.PeriodTo <= icuF.PeriodFrom || icuF.Count() != 6 {
		t.Fatalf("период брокерского отчёта должен попасть в файл: %+v", icuF)
	}
	before, _ := s.storage.CacheCounts()
	if before[icuF.AccountID] != 6 || before[db.BinanceAccountID] != 3 {
		t.Fatalf("counts %v", before)
	}

	if err := s.storage.WipeCache(); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.HandleLoadSources(rec, httptest.NewRequest(http.MethodPost, "/api/sources/load", nil))
	var res struct {
		Total, Loaded int
		Failed        []loadFailure
	}
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Loaded != 2 || len(res.Failed) != 0 {
		t.Fatalf("%+v %s", res, rec.Body.String())
	}
	if after, _ := s.storage.CacheCounts(); after[icuF.AccountID] != 6 || after[db.BinanceAccountID] != 3 {
		t.Fatalf("после загрузки %v", after)
	}
}

func TestSourceStoreDedupByContent(t *testing.T) {
	st := source.Store{Dir: t.TempDir()}
	f := func(name string) *source.File {
		return &source.File{Name: name, Origin: source.OriginCSV, Kind: source.KindBank, AccountID: "raif_X", Card: "4149****7777",
			Txs: nil, Rows: []source.BNRow{{T: 1000, Op: "x"}}}
	}
	if _, added, err := st.Put(f("a.csv")); err != nil || !added {
		t.Fatal(added, err)
	}
	if _, added, _ := st.Put(f("другое имя.csv")); added {
		t.Fatal("то же содержимое под другим именем — дубль")
	}
	g := f("a.csv")
	g.Rows = append(g.Rows, source.BNRow{T: 2000, Op: "y"})
	if _, added, _ := st.Put(g); !added {
		t.Fatal("другой период — новый файл")
	}
	list, bad := st.List()
	if len(list) != 2 || len(bad) != 0 || list[0].File.From != 1 || list[1].File.To != 2 {
		t.Fatalf("%+v %v", list, bad)
	}
}

func post(t *testing.T, h http.HandlerFunc, method, url string, body any) []byte {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(method, url, rd))
	if rec.Code != 200 {
		t.Fatalf("%s %s: %d %s", method, url, rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

// Данные API (банк): снимок с отрезками покрытия → сброс → загрузка возвращает и данные, и «уже снято».
func TestSaveAPIBankKeepsCoverage(t *testing.T) {
	t.Setenv("ECONOMIC_SOURCES_DIR", t.TempDir())
	e := newEnv(t)
	acc := mono.AccountDTO{ID: "acc1", Type: "black", CurrencyCode: 980, Balance: 12345, MaskedPan: []string{"5375****1111"}, IBAN: "UA00"}
	if err := e.st.SaveAccounts([]mono.AccountDTO{acc}, "Ivan"); err != nil {
		t.Fatal(err)
	}
	e.st.SaveTransactions("acc1", []mono.StatementItemDTO{
		{ID: "t1", Time: 1_700_000_000, Description: "кава", Amount: -5000, OperationAmount: -5000, CurrencyCode: 980},
		{ID: "t2", Time: 1_700_100_000, Description: "зп", Amount: 100000, OperationAmount: 100000, CurrencyCode: 980},
	})
	e.st.AddCoverage("acc1", db.HistoryStart, 1_700_200_000)

	var out struct{ Results []struct{ Saved bool } }
	json.Unmarshal(post(t, e.s.HandleSaveAPI, "POST", "/x", nil), &out)
	if len(out.Results) != 1 || !out.Results[0].Saved {
		t.Fatalf("%+v", out)
	}
	// повторно ничего нового — файл не плодится
	json.Unmarshal(post(t, e.s.HandleSaveAPI, "POST", "/x", nil), &out)
	if out.Results[0].Saved {
		t.Fatal("тот же снимок записан дважды")
	}
	if list, _ := store().List(); len(list) != 1 || list[0].File.Origin != source.OriginAPI || list[0].File.Name != "Ivan" {
		t.Fatalf("%+v", list)
	}

	e.st.WipeCache()
	post(t, e.s.HandleLoadSources, "POST", "/x", nil)
	cov, _ := e.st.Coverage("acc1")
	txs, _ := e.st.TxTimes("acc1")
	if len(cov) != 1 || cov[0].From != db.HistoryStart || cov[0].To != 1_700_200_000 || len(txs) != 2 {
		t.Fatalf("coverage %v txs %v", cov, txs)
	}
	accs, _ := e.st.GetAccounts()
	if len(accs) != 1 || accs[0].Source != "api" || accs[0].Balance != 123.45 {
		t.Fatalf("счёт остаётся «живым»: %+v", accs)
	}

	// снести снимок: кеш забывает, что снято → следующий синк стянет всё заново
	list, _ := store().List()
	var del struct{ Refetch bool }
	json.Unmarshal(post(t, e.s.HandleDeleteSource, "DELETE", "/x?id="+list[0].File.Hash, nil), &del)
	if cov, _ = e.st.Coverage("acc1"); len(cov) != 0 || !del.Refetch {
		t.Fatalf("после удаления снимка покрытие должно исчезнуть: %v %+v", cov, del)
	}
	if l, _ := store().List(); len(l) != 0 {
		t.Fatal("файл остался")
	}
}

// Binance: минутный ряд — тоже источник; после сброса и загрузки «Разведать» дописывает только хвост.
func TestSaveAPIBinanceSeriesRestores(t *testing.T) {
	t.Setenv("ECONOMIC_SOURCES_DIR", t.TempDir())
	now := time.Now().UTC().Truncate(time.Minute)
	fake := newPriceFake()
	defer fake.Close()
	s, st := newBinanceServer(t, fake)
	if _, err := s.importSource(stmtName, []byte(stmtFor(now.Add(-3*time.Hour), now.Add(-2*time.Hour)))); err != nil {
		t.Fatal(err)
	}
	start := now.Add(-3 * time.Hour).Unix()
	vals := []float64{10, 11, 12.5, 13}
	if err := st.SaveBinance(db.BinanceSave{Portfolio: 13, Start: start, Vals: vals, Replace: true, Origin: 7, Detail: map[string]any{"total": 13, "wallets": []any{}}}); err != nil {
		t.Fatal(err)
	}
	st.SaveBinanceTxs([]db.BinanceTx{{FlowID: "f1", T: start * 1000, Desc: "Binance: Пополнение 1 USDT", UAH: 41.5}})

	var out struct{ Results []struct{ Saved bool } }
	json.Unmarshal(post(t, s.HandleSaveAPI, "POST", "/x", map[string]any{"accounts": []string{db.BinanceAccountID}}), &out)
	if len(out.Results) != 1 || !out.Results[0].Saved {
		t.Fatalf("%+v", out)
	}
	// файл статьи и файл ряда лежат рядом: выписка + API
	list, _ := store().List()
	kinds := map[string]bool{}
	for _, e := range list {
		kinds[e.File.Kind] = true
	}
	if !kinds[source.KindSeries] {
		t.Fatalf("%v", kinds)
	}

	st.WipeCache()
	var load struct {
		Loaded int
		Failed []loadFailure
	}
	// выписки в хранилище нет (импорт шёл мимо HandleImportCSV) — положим её, как это сделал бы импорт
	f, _ := parseSource(stmtName, []byte(stmtFor(now.Add(-3*time.Hour), now.Add(-2*time.Hour))))
	store().Put(f)
	json.Unmarshal(post(t, s.HandleLoadSources, "POST", "/x", nil), &load)
	if load.Loaded != 2 || len(load.Failed) != 0 {
		t.Fatalf("%+v", load)
	}
	first, last, n := st.BinanceSeriesInfo()
	if first != start || last != start+180 || n != 4 {
		t.Fatalf("ряд: %d %d %d", first, last, n)
	}
	if _, lv, _ := st.BinanceLastMinute(); lv != 13 {
		t.Fatalf("last %v", lv)
	}
	if len(st.BinanceDetail()) == 0 {
		t.Fatal("разбор по монетам (карточка) должен вернуться")
	}
	if txs, _ := st.TxTimes(db.BinanceAccountID); len(txs) != 1 {
		t.Fatalf("оценённые движения должны вернуться: %v", txs)
	}

	// другая выписка → ряд устарел и не загружается молча
	st.WipeCache()
	f2, _ := parseSource(stmtName, []byte(stmtFor(now.Add(-5*time.Hour), now.Add(-4*time.Hour))))
	cur, _ := store().List()
	for _, e := range cur {
		if e.File.Kind != source.KindSeries {
			store().Remove(e.Path)
		}
	}
	store().Put(f2)
	json.Unmarshal(post(t, s.HandleLoadSources, "POST", "/x", nil), &load)
	if load.Loaded != 1 || len(load.Failed) != 1 {
		t.Fatalf("устаревший ряд: %+v", load)
	}
	if _, _, n := st.BinanceSeriesInfo(); n != 0 {
		t.Fatal("устаревший ряд не должен попасть в кеш")
	}
}

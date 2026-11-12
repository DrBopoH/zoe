package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"economic/pkg/binance"
	"economic/pkg/db"
	"economic/pkg/icu"
	"economic/pkg/mono"
	"economic/pkg/privat"
	"economic/pkg/raif"
	"economic/pkg/source"
)

func store() source.Store { return source.Store{Dir: sourcesDir()} }

// parseSource разбирает выписку (PDF Привата / CSV Райфа / выписка Binance / брокерский отчёт ICU) в единый файл-источник.
// Ничего никуда не пишет.
func parseSource(filename string, raw []byte) (*source.File, error) {
	switch {
	case binance.IsStatement(raw):
		st, err := binance.ParseStatement(bytes.NewReader(raw), binance.StatementZone(filename))
		if err != nil {
			return nil, &importError{http.StatusBadRequest, "Failed to parse Binance statement: " + err.Error()}
		}
		f := &source.File{Name: filename, Origin: source.OriginBinance, Kind: source.KindBinance,
			AccountID: db.BinanceAccountID, Card: "binance", Profile: binance.ProfileID(st.UserID)}
		for _, r := range st.Rows {
			f.Rows = append(f.Rows, source.BNRow{T: r.T, Account: r.Account, Op: r.Op, Asset: r.Asset, Delta: r.Delta, Note: r.Note})
		}
		return f, nil

	case icu.IsReport(raw):
		rep, err := icu.ParseReport(raw)
		if err != nil {
			return nil, &importError{http.StatusBadRequest, "Failed to parse ICU report: " + err.Error()}
		}
		return icuFile(filename, rep), nil
	}

	var targetIBAN, targetCard, bankType string
	var balance float64
	var transactions []mono.StatementItemDTO
	origin := source.OriginCSV
	if bytes.HasPrefix(raw, []byte("%PDF")) || strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		parsed, err := privat.ParsePDF(bytes.NewReader(raw))
		if err != nil {
			return nil, &importError{http.StatusBadRequest, fmt.Sprintf("Failed to parse Privat PDF: %v", err)}
		}
		targetIBAN, targetCard, bankType, balance, transactions = parsed.IBAN, parsed.CardNumber, "privat", parsed.Balance, parsed.Transactions
		origin = source.OriginPDF
	} else {
		parsed, err := raif.ParseCSV(bytes.NewReader(raw))
		if err != nil {
			return nil, &importError{http.StatusBadRequest, fmt.Sprintf("Failed to parse CSV: %v", err)}
		}
		targetIBAN, targetCard, bankType, balance, transactions = parsed.IBAN, parsed.CardNumber, parsed.BankType, parsed.Balance, parsed.Transactions
	}
	if len(transactions) == 0 {
		return nil, &importError{http.StatusBadRequest, "No transactions found in file"}
	}

	accountID := fmt.Sprintf("%s_%s", bankType, targetIBAN)
	if targetIBAN == "" && targetCard != "" {
		accountID = fmt.Sprintf("%s_%s", bankType, targetCard)
	}
	var pans []string
	if targetCard != "" {
		pans = append(pans, targetCard)
	}
	currency := 980
	if transactions[0].CurrencyCode != 0 {
		currency = transactions[0].CurrencyCode
	}
	return &source.File{Name: filename, Origin: origin, Kind: source.KindBank, AccountID: accountID, Card: targetCard,
		Account: &mono.AccountDTO{ID: accountID, IBAN: targetIBAN, MaskedPan: pans, Type: bankType, CurrencyCode: currency,
			Balance: int64(math.Round(balance * 100))},
		Txs: transactions}, nil
}

// icuFile — брокерский отчёт в формате источника: счёт (ключ = номер договора) и журнал.
func icuFile(filename string, rep *icu.Report) *source.File {
	accID := db.ICUAccountID(rep.Contract)
	f := &source.File{Name: filename, Origin: source.OriginReport, Kind: source.KindICU, AccountID: accID, Card: db.ICUAccountType,
		Account:  &mono.AccountDTO{ID: accID, Type: db.ICUAccountType, CurrencyCode: 980, Balance: int64(math.Round(rep.Cash.End * 100))}, // баланс считается на чтении; здесь — свободные деньги на конец периода
		Contract: rep.Contract, ContractDate: rep.ContractDate.Unix(), CashEnd: rep.Cash.End,
		PeriodFrom: rep.From.Unix(), PeriodTo: rep.To.Unix()}
	for _, fl := range rep.Flows {
		f.Flows = append(f.Flows, source.Flow{ID: fl.ID, Time: fl.Time, Kind: fl.Kind, Amount: fl.Amount, Note: fl.Note, ISIN: fl.ISIN, Qty: fl.Qty})
	}
	return f
}

// applySource кладёт файл-источник в кеш-базу. Единый путь и для импорта, и для «Загрузить».
func (s *Server) applySource(f *source.File) (*importResult, error) {
	if f.Format == "" { // только что разобранный файл ещё не запечатан
		f.Seal()
	}
	if err := f.Validate(); err != nil {
		return nil, &importError{http.StatusBadRequest, err.Error()}
	}
	fail := func(what string, err error) (*importResult, error) {
		return nil, &importError{http.StatusInternalServerError, what + ": " + err.Error()}
	}
	switch f.Kind {
	case source.KindBinance:
		rows := make([]db.BinanceRow, len(f.Rows))
		for i, r := range f.Rows {
			br := binance.Row{T: r.T, Account: r.Account, Op: r.Op, Asset: r.Asset, Delta: r.Delta, Note: r.Note}
			rows[i] = db.BinanceRow{T: r.T, Account: r.Account, Op: r.Op, Asset: r.Asset, Delta: r.Delta, Kind: binance.Kind(br), Note: r.Note} // вид операции — по текущим правилам
		}
		if err := s.storage.SaveBinanceStatement(f.Name, f.Profile, rows); err != nil {
			return fail("Failed to save Binance statement", err)
		}
		return &importResult{AccountID: db.BinanceAccountID, Card: "binance", Count: len(rows)}, nil

	case source.KindICU:
		if f.Account == nil {
			return nil, &importError{http.StatusBadRequest, "В источнике нет счёта"}
		}
		if err := s.storage.SaveImportedAccount(*f.Account, f.Name); err != nil {
			return fail("Failed to save ICU account", err)
		}
		rows := make([]db.LedgerRow, len(f.Flows))
		for i, fl := range f.Flows {
			rows[i] = db.LedgerRow{ID: fl.ID, AccountID: f.AccountID, Time: fl.Time, Kind: fl.Kind, Amount: fl.Amount, Note: fl.Note, ISIN: fl.ISIN, Qty: fl.Qty}
		}
		if err := s.storage.SaveICUFlows(f.AccountID, f.ContractDate, rows); err != nil {
			return fail("Failed to save ICU journal", err)
		}
		return &importResult{AccountID: f.AccountID, Card: db.ICUAccountType, Count: len(rows)}, nil

	case source.KindSeries:
		sr := f.Series
		if sr == nil || len(sr.Vals) == 0 {
			return nil, &importError{http.StatusBadRequest, "В источнике нет ряда"}
		}
		if cur := s.ledgerSig(); cur == "" || cur != sr.Sig {
			// журнал выписки другой (или ещё не загружен): ряд, построенный на старом, дал бы неверный график
			return nil, &importError{http.StatusConflict, "ряд построен для другой выписки — при «Разведать» соберётся заново"}
		}
		if err := s.storage.SaveTransactions(db.BinanceAccountID, f.Txs); err != nil { // оценки движений в ₴ (баланс ниже перезапишется)
			return fail("Failed to save Binance operations", err)
		}
		if err := s.storage.SaveBinance(db.BinanceSave{Portfolio: sr.Portfolio, Start: sr.Start, Vals: sr.Vals, Replace: true, Origin: sr.Origin, Detail: sr.Detail}); err != nil {
			return fail("Failed to save Binance series", err)
		}
		return &importResult{AccountID: db.BinanceAccountID, Card: "binance", Count: len(sr.Vals)}, nil

	case source.KindBank:
		if f.Account == nil {
			return nil, &importError{http.StatusBadRequest, "В источнике нет счёта"}
		}
		if f.Origin == source.OriginAPI { // снимок API: счёт остаётся «живым» (баланс банка, главная карта), снятые отрезки не опрашиваются заново
			if err := s.storage.SaveAccounts([]mono.AccountDTO{*f.Account}, f.Name); err != nil {
				return fail("Failed to save account", err)
			}
		} else if err := s.storage.SaveImportedAccount(*f.Account, f.Name); err != nil {
			return fail("Failed to save account", err)
		}
		if err := s.storage.SaveTransactions(f.AccountID, f.Txs); err != nil {
			return fail("Failed to save transactions", err)
		}
		for _, sp := range f.Spans {
			if err := s.storage.AddCoverage(f.AccountID, sp[0], sp[1]); err != nil {
				return fail("Failed to save coverage", err)
			}
		}
		return &importResult{AccountID: f.AccountID, IBAN: f.Account.IBAN, Card: f.Card, Count: len(f.Txs)}, nil
	}
	return nil, &importError{http.StatusBadRequest, "Неизвестный вид источника: " + f.Kind}
}

// importSource разбирает выписку и пишет её в кеш-базу (файл-источник не создаётся — это делает HandleImportCSV).
func (s *Server) importSource(filename string, raw []byte) (*importResult, error) {
	f, err := parseSource(filename, raw)
	if err != nil {
		return nil, err
	}
	return s.applySource(f)
}

// migrateLegacy переводит старые «сырые» файлы кэша источников (<хэш>_<имя>) в формат .zoe.
// Оригиналы уезжают в служебную папку _legacy; что не разобралось — остаётся на месте и попадает в отчёт.
func migrateLegacy() (moved int, failed []loadFailure) {
	root := sourcesDir()
	dirs, err := os.ReadDir(root)
	if err != nil {
		return 0, nil
	}
	for _, d := range dirs {
		if !d.IsDir() || strings.HasPrefix(d.Name(), "_") {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(root, d.Name()))
		for _, e := range files {
			name := e.Name()
			if e.IsDir() || strings.HasSuffix(name, source.Ext) || strings.HasSuffix(name, ".tmp") {
				continue
			}
			p := filepath.Join(root, d.Name(), name)
			orig := name
			if i := strings.Index(name, "_"); i == 20 {
				orig = name[i+1:]
			}
			raw, err := os.ReadFile(p)
			if err == nil {
				var f *source.File
				if f, err = parseSource(orig, raw); err == nil {
					f.Created = fileTime(p)
					_, _, err = store().Put(f)
				}
			}
			if err != nil {
				failed = append(failed, loadFailure{File: orig, Error: err.Error()})
				continue
			}
			dst := filepath.Join(root, "_legacy", d.Name())
			if os.MkdirAll(dst, 0o700) == nil && os.Rename(p, filepath.Join(dst, name)) == nil {
				moved++
			}
		}
	}
	return moved, failed
}

func fileTime(p string) int64 {
	if fi, err := os.Stat(p); err == nil {
		return fi.ModTime().Unix()
	}
	return 0
}

// ledgerSig — подпись журнала Binance в кеше ("" — журнала нет). Ряд оценки годится, только пока журнал тот же.
func (s *Server) ledgerSig() string {
	rows, err := s.storage.BinanceLedger()
	if err != nil || len(rows) == 0 {
		return ""
	}
	h := sha256.New()
	for _, r := range rows {
		fmt.Fprintf(h, "%d|%s|%s|%s|%.12g\n", r.T, r.Account, r.Op, r.Asset, r.Delta)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// snapshotAPI собирает файл-источник из того, что API уже отдал и кеш запомнил. nil — снимать нечего.
func (s *Server) snapshotAPI(acc db.Account) (*source.File, error) {
	if acc.ID == db.BinanceAccountID {
		start, vals, origin, hasOrigin, portfolio, ok := s.storage.BinanceSeriesDump()
		if !ok {
			return nil, nil
		}
		txs, err := s.storage.AccountTxs(acc.ID, "binance_") // оценки движений в ₴ — тоже запросы к API
		if err != nil {
			return nil, err
		}
		return &source.File{Name: "Binance", Origin: source.OriginAPI, Kind: source.KindSeries, AccountID: acc.ID, Card: "binance", Txs: txs,
			Series: &source.Series{Start: start, Vals: vals, Origin: origin, HasOrigin: hasOrigin, Portfolio: portfolio, Sig: s.ledgerSig(), Detail: s.storage.BinanceDetail()}}, nil
	}
	if acc.Source != "api" {
		return nil, nil
	}
	spans, err := s.storage.Coverage(acc.ID)
	if err != nil || len(spans) == 0 {
		return nil, err
	}
	txs, err := s.storage.AccountTxs(acc.ID, "")
	if err != nil {
		return nil, err
	}
	pans := append([]string(nil), acc.Pans...)
	f := &source.File{Name: s.storage.AccountOwner(acc.ID), Origin: source.OriginAPI, Kind: source.KindBank, AccountID: acc.ID, Card: acc.MaskedPan, Txs: txs,
		Account: &mono.AccountDTO{ID: acc.ID, IBAN: acc.IBAN, MaskedPan: pans, Type: acc.Type, CurrencyCode: acc.Currency,
			Balance: int64(math.Round(acc.Balance * 100)), CreditLimit: int64(math.Round(acc.CreditLimit * 100))}}
	if f.Name == "" {
		f.Name = acc.Name
	}
	for _, sp := range spans {
		f.Spans = append(f.Spans, [2]int64{sp.From, sp.To})
	}
	return f, nil
}

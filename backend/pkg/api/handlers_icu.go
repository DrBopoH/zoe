package api

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"

	"economic/pkg/db"
	"economic/pkg/icu"
	"economic/pkg/mono"
)

func icuJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// importICUReport — брокерский отчёт ICU → счёт и журнал в кеше. Идёт тем же путём, что выписки банков
// (importSource → кэш источников → «Загрузить»): ничего не вводится руками, привязка к лицу — алиас по имени файла.
func (s *Server) importICUReport(filename string, raw []byte) (*importResult, error) {
	rep, err := icu.ParseReport(raw)
	if err != nil {
		return nil, &importError{http.StatusBadRequest, "Failed to parse ICU report: " + err.Error()}
	}
	return s.saveICUReport(filename, rep)
}

// saveICUReport кладёт разобранный отчёт в кеш: счёт (имя = имя файла, ключ = номер договора) и журнал.
func (s *Server) saveICUReport(filename string, rep *icu.Report) (*importResult, error) {
	accID := db.ICUAccountID(rep.Contract)
	acc := mono.AccountDTO{
		ID: accID, Type: db.ICUAccountType, CurrencyCode: 980,
		Balance: int64(math.Round(rep.Cash.End * 100)), // баланс счёта считается на чтении; здесь — свободные деньги на конец периода
	}
	if err := s.storage.SaveImportedAccount(acc, filename); err != nil {
		return nil, &importError{http.StatusInternalServerError, "Failed to save ICU account: " + err.Error()}
	}
	rows := make([]db.LedgerRow, len(rep.Flows))
	for i, f := range rep.Flows {
		rows[i] = db.LedgerRow{ID: f.ID, AccountID: accID, Time: f.Time, Kind: f.Kind, Amount: f.Amount, Note: f.Note, ISIN: f.ISIN, Qty: f.Qty}
	}
	if err := s.storage.SaveICUFlows(accID, rep.ContractDate.Unix(), rows); err != nil {
		return nil, &importError{http.StatusInternalServerError, "Failed to save ICU journal: " + err.Error()}
	}
	return &importResult{AccountID: accID, Card: db.ICUAccountType, Count: len(rows)}, nil
}

// GET /api/icu/ledger?account_id=… — журнал, позиции и выплаты по графику брокерского счёта (только чтение).
func (s *Server) HandleICULedger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ov, err := s.storage.ICUOverview(r.URL.Query().Get("account_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	icuJSON(w, ov)
}

// /api/icu/bonds — справочник облигаций (ISIN, погашение, ставка, номинал). В отчёте брокера этих данных нет.
//
//	GET
//	POST   — создать/обновить (isin, name, maturity YYYY-MM-DD, coupon_rate %, nominal)
//	DELETE ?isin=…
func (s *Server) HandleICUBonds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.ListBonds()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		icuJSON(w, list)
	case http.MethodPost, http.MethodPut:
		var b db.Bond
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		saved, err := s.storage.SaveBond(b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		icuJSON(w, saved)
	case http.MethodDelete:
		if err := s.storage.DeleteBond(strings.ToUpper(r.URL.Query().Get("isin"))); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		icuJSON(w, map[string]string{"status": "success"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

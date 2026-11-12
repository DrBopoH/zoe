package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"economic/pkg/db"
	"economic/pkg/icu"
)

func icuJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// saveICUReport кладёт разобранный отчёт в кеш (тот же путь, что у файла-источника).
func (s *Server) saveICUReport(filename string, rep *icu.Report) (*importResult, error) {
	return s.applySource(icuFile(filename, rep))
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

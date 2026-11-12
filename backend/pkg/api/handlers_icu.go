package api

import (
	"net/http"
	"strings"

	"economic/pkg/db"
	"economic/pkg/icu"
)

// saveICUReport кладёт разобранный отчёт в кеш (тот же путь, что у файла-источника).
func (s *Server) saveICUReport(filename string, rep *icu.Report) (*importResult, error) {
	return s.applySource(icuFile(filename, rep))
}

// GET /api/icu/ledger?account_id=… — журнал, позиции и выплаты по графику брокерского счёта (только чтение).
func (s *Server) HandleICULedger(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	ov, err := s.storage.ICUOverview(r.URL.Query().Get("account_id"))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, ov)
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
			failErr(w, err)
			return
		}
		writeJSON(w, list)
	case http.MethodPost, http.MethodPut:
		var b db.Bond
		if !readJSON(w, r, &b) {
			return
		}
		saved, err := s.storage.SaveBond(b)
		if err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, saved)
	case http.MethodDelete:
		if err := s.storage.DeleteBond(strings.ToUpper(r.URL.Query().Get("isin"))); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "success"})
	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

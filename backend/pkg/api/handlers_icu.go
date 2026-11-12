package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"economic/pkg/db"
)

func icuJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// /api/manual-accounts — ручные счета (ICU Trader и т.п.), живут в core.db.
//
//	GET    — список
//	POST   — создать/обновить (тело: id?, name, iban, type?, currency_code?, bank_key?)
//	DELETE — ?id=…  (вместе с журналом счёта)
func (s *Server) HandleManualAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.ListManualAccounts()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		icuJSON(w, list)
	case http.MethodPost, http.MethodPut:
		var m db.ManualAccount
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		saved, err := s.storage.SaveManualAccount(m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		icuJSON(w, saved)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		if err := s.storage.DeleteManualAccount(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		icuJSON(w, map[string]string{"status": "success"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type ledgerRequest struct {
	ID        int64   `json:"id"`
	AccountID string  `json:"account_id"`
	Date      string  `json:"date"` // YYYY-MM-DD (полдень по местному времени), либо YYYY-MM-DDTHH:MM, либо RFC3339
	Time      int64   `json:"time"` // unix — если date не задана
	Kind      string  `json:"kind"`
	Amount    float64 `json:"amount"`
	Note      string  `json:"note"`
	SrcTxID   string  `json:"src_tx_id"`
	ISIN      string  `json:"isin"` // buy/sell
	Qty       float64 `json:"qty"`  // buy/sell
}

func parseLedgerTime(date string, unix int64) (int64, bool) {
	date = strings.TrimSpace(date)
	if date == "" {
		return unix, unix > 0
	}
	if t, err := time.ParseInLocation("2006-01-02", date, time.Local); err == nil {
		return t.Add(12 * time.Hour).Unix(), true
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", date, time.Local); err == nil {
		return t.Unix(), true
	}
	if t, err := time.Parse(time.RFC3339, date); err == nil {
		return t.Unix(), true
	}
	return 0, false
}

// /api/icu/ledger — журнал ручного счёта.
//
//	GET    ?account_id=…  — журнал с цветом сверки и «призраками» (операции кеша без записи)
//	POST   — добавить/изменить запись (id == 0 — новая)
//	DELETE ?id=…          — удалить запись
func (s *Server) HandleICULedger(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ov, err := s.storage.ICUOverview(r.URL.Query().Get("account_id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		icuJSON(w, ov)
	case http.MethodPost, http.MethodPut:
		var req ledgerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		t, ok := parseLedgerTime(req.Date, req.Time)
		if !ok {
			http.Error(w, "Некорректная дата", http.StatusBadRequest)
			return
		}
		row, err := s.storage.SaveLedgerRow(db.LedgerRow{
			ID: req.ID, AccountID: req.AccountID, Time: t, Kind: req.Kind,
			Amount: req.Amount, Note: req.Note, SrcTxID: req.SrcTxID, ISIN: req.ISIN, Qty: req.Qty,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		icuJSON(w, row)
	case http.MethodDelete:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		if err := s.storage.DeleteLedgerRow(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		icuJSON(w, map[string]string{"status": "success"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type acceptRequest struct {
	AccountID string   `json:"account_id"`
	TxIDs     []string `json:"tx_ids"`
	All       bool     `json:"all"`
}

// POST /api/icu/accept — перенести операции кеша («призраков») в журнал: выбранные tx_ids или all.
func (s *Server) HandleICUAccept(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req acceptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AccountID == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	n, err := s.storage.AcceptGhosts(req.AccountID, req.TxIDs, req.All)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	icuJSON(w, map[string]int{"accepted": n})
}

// /api/icu/bonds — «тела» облигаций счёта ICU (ISIN, погашение, ставка, номинал). Счёт задаётся account_id или iban.
//
//	GET    ?iban=… | ?account_id=…
//	POST   — создать/обновить (iban|account_id, isin, name, maturity YYYY-MM-DD, coupon_rate %, nominal)
//	DELETE ?iban=…&isin=…
func (s *Server) HandleICUBonds(w http.ResponseWriter, r *http.Request) {
	accountOf := func(accountID, iban string) (string, error) {
		if accountID != "" {
			return accountID, nil
		}
		m, err := s.storage.ICUAccountByIBAN(iban)
		return m.ID, err
	}
	switch r.Method {
	case http.MethodGet:
		id, err := accountOf(r.URL.Query().Get("account_id"), r.URL.Query().Get("iban"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		list, err := s.storage.ListBonds(id)
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
		id, err := accountOf(b.AccountID, b.IBAN)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		b.AccountID = id
		saved, err := s.storage.SaveBond(b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		icuJSON(w, saved)
	case http.MethodDelete:
		q := r.URL.Query()
		id, err := accountOf(q.Get("account_id"), q.Get("iban"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.storage.DeleteBond(id, strings.ToUpper(q.Get("isin"))); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		icuJSON(w, map[string]string{"status": "success"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

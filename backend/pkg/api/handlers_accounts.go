package api

import (
	"net/http"
	"strconv"
	"time"
)

// Счета, транзакции, статус и запуск синхронизации, сброс кеша.

type UpdateAccountManualRequest struct {
	AccountID   string  `json:"account_id"`
	IBAN        string  `json:"iban"`
	CardNumber  string  `json:"card_number"`
	CreditLimit float64 `json:"credit_limit"`
}

func (s *Server) HandleUpdateAccountManual(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, writeMethods...) {
		return
	}
	var req UpdateAccountManualRequest
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.storage.UpdateAccountManualDetails(req.AccountID, req.IBAN, req.CardNumber, req.CreditLimit); err != nil {
		failErr(w, err)
		return
	}
	writeOK(w)
}

func (s *Server) HandleGetAccounts(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	accounts, err := s.storage.GetAllAccounts() // счета кеша + ручные (ICU)
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, accounts)
}

func (s *Server) HandleGetTransactions(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	limit := 50000
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 500000 {
		limit = v
	}
	txs, err := s.storage.GetAllTransactions(limit) // кеш + записи журналов ручных счетов
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, txs)
}

func (s *Server) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	s.syncMutex.Lock()
	running := s.isSyncing
	statusMsg := s.syncStatus
	s.syncMutex.Unlock()

	stats, _ := s.storage.GetSyncStats()

	writeJSON(w, map[string]interface{}{
		"running":  running,
		"status":   statusMsg,
		"stats":    stats,
		"progress": s.progressSnapshot(),
	})
}

func (s *Server) HandleFullHistoricalSync(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}
	ctx, done, ok := s.beginSync()
	if !ok {
		fail(w, http.StatusConflict, "Синхронизация уже выполняется")
		return
	}
	go s.runFullCrawler(ctx, done)

	writeMsg(w, "started", "Полная историческая синхронизация запущена.")
}

type UpdateCreditLimitRequest struct {
	AccountID   string  `json:"account_id"`
	CreditLimit float64 `json:"credit_limit"`
}

func (s *Server) HandleUpdateCreditLimit(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, writeMethods...) {
		return
	}
	var req UpdateCreditLimitRequest
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.storage.UpdateCreditLimit(req.AccountID, req.CreditLimit); err != nil {
		failErr(w, err)
		return
	}
	writeOK(w)
}

type accountLifeRequest struct {
	AccountID string `json:"account_id"`
	Opened    string `json:"opened"` // YYYY-MM-DD или "" (по выписке)
	Closed    string `json:"closed"` // YYYY-MM-DD или "" (не закрыт / по данным банка)
}

// HandleSetAccountLife — ручные даты открытия/закрытия счёта (банк их не отдаёт).
func (s *Server) HandleSetAccountLife(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, writeMethods...) {
		return
	}
	var req accountLifeRequest
	if !readJSON(w, r, &req) {
		return
	}
	if req.AccountID == "" {
		fail(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	parse := func(v string) (int64, error) {
		if v == "" {
			return 0, nil
		}
		t, err := time.Parse("2006-01-02", v)
		return t.Unix(), err
	}
	opened, err1 := parse(req.Opened)
	closed, err2 := parse(req.Closed)
	if err1 != nil || err2 != nil || (opened > 0 && closed > 0 && closed < opened) {
		fail(w, http.StatusBadRequest, "Некорректные даты")
		return
	}
	if err := s.storage.SetAccountLife(req.AccountID, opened, closed); err != nil {
		failErr(w, err)
		return
	}
	writeOK(w)
}

func (s *Server) HandleWipeCache(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}
	s.abortSync() // идущий «Разведать» снимается: иначе он дописал бы данные в чистую базу, а кнопка так и висела бы «РАЗВЕДЫВАЮ…»
	if err := s.storage.WipeCache(); err != nil {
		failErr(w, err)
		return
	}
	writeMsg(w, "success", "Кэш выписок очищен. База графа и лиц (core.db) в полной сохранности.")
}

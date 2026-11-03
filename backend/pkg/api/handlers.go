package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"economic/pkg/db"
	"economic/pkg/mono"
)

type Server struct {
	storage    *db.Storage
	monoClient *mono.Client

	syncMutex  sync.Mutex
	isSyncing  bool
	syncStatus string
}

func NewServer(storage *db.Storage, monoClient *mono.Client) *Server {
	return &Server{
		storage:    storage,
		monoClient: monoClient,
		syncStatus: "IDLE",
	}
}

func (s *Server) EnableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func (s *Server) HandleGetAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.storage.GetAccounts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accounts)
}

func (s *Server) HandleGetTransactions(w http.ResponseWriter, r *http.Request) {
	txs, err := s.storage.GetTransactions(10000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(txs)
}

func (s *Server) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	s.syncMutex.Lock()
	running := s.isSyncing
	statusMsg := s.syncStatus
	s.syncMutex.Unlock()

	stats, _ := s.storage.GetSyncStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"running": running,
		"status":  statusMsg,
		"stats":   stats,
	})
}

func (s *Server) HandleFullHistoricalSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.syncMutex.Lock()
	if s.isSyncing {
		s.syncMutex.Unlock()
		http.Error(w, "Синхронизация уже выполняется", http.StatusConflict)
		return
	}
	s.isSyncing = true
	s.syncStatus = "Инициализация API..."
	s.syncMutex.Unlock()

	go s.runFullCrawler()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Полная историческая синхронизация запущена.",
	})
}

func (s *Server) runFullCrawler() {
	defer func() {
		s.syncMutex.Lock()
		s.isSyncing = false
		s.syncStatus = "COMPLETED"
		s.syncMutex.Unlock()
	}()

	info, err := s.monoClient.GetClientInfo()
	if err != nil {
		log.Printf("[Sync Error] GetClientInfo: %v", err)
		s.updateStatus("Ошибка авторизации API")
		return
	}

	_ = s.storage.SaveAccounts(info.Accounts, info.Name)

	if len(info.Accounts) == 0 {
		s.updateStatus("Счета не найдены")
		return
	}

	for accIndex, acc := range info.Accounts {
		s.updateStatus(fmt.Sprintf("Сбор аккаунта %d/%d (%s)", accIndex+1, len(info.Accounts), acc.Type))

		to := time.Now()

		for {
			from := to.AddDate(0, 0, -30)

			s.updateStatus(fmt.Sprintf("[%s] Запрос окна %s — %s",
				acc.Type,
				from.Format("02.01.2006"),
				to.Format("02.01.2006"),
			))

			statement, err := s.monoClient.GetStatement(acc.ID, from, to)

			if _, isRateErr := err.(*mono.RateLimitError); isRateErr {
				s.updateStatus("Превышен лимит API (429). Ожидание 65 секунд...")
				time.Sleep(65 * time.Second)
				continue
			}

			if err != nil {
				log.Printf("[Sync Error] Statement err: %v", err)
				s.updateStatus("Ошибка запроса выписки")
				break
			}

			if len(statement) == 0 {
				log.Printf("[Sync] Достигнут конец истории для счета %s", acc.ID)
				break
			}

			if err := s.storage.SaveTransactions(acc.ID, statement); err != nil {
				log.Printf("[Sync Error] DB Save: %v", err)
				break
			}

			oldestUnix := statement[len(statement)-1].Time
			to = time.Unix(oldestUnix, 0).Add(-1 * time.Second)

			s.updateStatus(fmt.Sprintf("Загружено %d транзакций. Кулдаун 61с...", len(statement)))
			time.Sleep(61 * time.Second)
		}
	}
}

func (s *Server) updateStatus(msg string) {
	s.syncMutex.Lock()
	s.syncStatus = msg
	s.syncMutex.Unlock()
	log.Printf("[Sync Progress] %s", msg)
}

type UpdateCreditLimitRequest struct {
	AccountID   string  `json:"account_id"`
	CreditLimit float64 `json:"credit_limit"`
}

func (s *Server) HandleUpdateCreditLimit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateCreditLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if err := s.storage.UpdateCreditLimit(req.AccountID, req.CreditLimit); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
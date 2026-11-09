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

func (s *Server) updateStatus(msg string) {
	s.syncMutex.Lock()
	s.syncStatus = msg
	s.syncMutex.Unlock()
	log.Printf("[Sync Status] %s", msg)
}

// Человекочитаемое форматирование даты N (день) name (месяц) YYYY
var ruMonthsShort = []string{
	"", "янв.", "февр.", "марта", "апр.", "мая", "июня",
	"июля", "авг.", "сент.", "окт.", "нояб.", "декабря",
}

func formatHumanDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), ruMonthsShort[t.Month()], t.Year())
}

func (s *Server) EnableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func (s *Server) runFullCrawler() {
	defer func() {
		s.syncMutex.Lock()
		s.isSyncing = false
		s.syncStatus = "СНЯТО"
		s.syncMutex.Unlock()
	}()

	info, err := s.monoClient.GetClientInfo()
	if err != nil {
		log.Printf("[Sync Error] GetClientInfo: %v", err)
		s.updateStatus("ОТКАЗ КЛЮЧА API")
		return
	}

	_ = s.storage.SaveAccounts(info.Accounts, info.Name)

	if len(info.Accounts) == 0 {
		s.updateStatus("СЧЕТА ПУСТЫ")
		return
	}

	minHistoricalDate := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)

	for accIndex, acc := range info.Accounts {
		s.updateStatus(fmt.Sprintf("СЧЕТ %d/%d (%s)", accIndex+1, len(info.Accounts), acc.Type))

		to := time.Now()
		emptyWindowsCount := 0

		for {
			from := to.AddDate(0, 0, -30)

			if from.Before(minHistoricalDate) {
				log.Printf("[Sync] Достигнут лимит истории (2017)")
				break
			}

			statement, err := s.monoClient.GetStatement(acc.ID, from, to)

			if _, isRateErr := err.(*mono.RateLimitError); isRateErr {
				s.updateStatus("ЛИМИТ БАНКА (429) · ОХЛАЖДЕНИЕ 65С")
				time.Sleep(65 * time.Second)
				continue
			}

			if _, isBadRequest := err.(*mono.BadRequestError); isBadRequest {
				log.Printf("[Sync] Финиш истории счета %s", acc.ID)
				s.updateStatus(fmt.Sprintf("[%s] ИСТОРИЯ ПОЛНАЯ", acc.Type))
				break
			}

			if err != nil {
				log.Printf("[Sync Error] Statement err: %v", err)
				s.updateStatus("СБОЙ ВЫПИСКИ")
				break
			}

			// ПУСТОЕ ОКНО — Явно выводим статус с кулдауном перед сном
			if len(statement) == 0 {
				emptyWindowsCount++
				if emptyWindowsCount >= 12 {
					break
				}
				to = from.Add(-1 * time.Second)
				s.updateStatus(fmt.Sprintf("[%s] %s — %s · ОХЛАЖДЕНИЕ 61С", acc.Type, formatHumanDate(from), formatHumanDate(to)))
				time.Sleep(61 * time.Second)
				continue
			}

			emptyWindowsCount = 0

			if err := s.storage.SaveTransactions(acc.ID, statement); err != nil {
				log.Printf("[Sync Error] DB Save: %v", err)
				break
			}

			oldestUnix := statement[len(statement)-1].Time
			to = time.Unix(oldestUnix, 0).Add(-1 * time.Second)

			s.updateStatus(fmt.Sprintf("СНЯТО %d ТРАНШЕЙ · ОХЛАЖДЕНИЕ 61С", len(statement)))
			time.Sleep(61 * time.Second)
		}
	}
}

// --- USERS HANDLERS (FULL CRUD) ---

func (s *Server) HandleUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		users, err := s.storage.GetUsers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(users)

	case http.MethodPost:
		var u db.User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if u.ID == "" {
			u.ID = fmt.Sprintf("user_%d", time.Now().UnixNano())
		}
		if err := s.storage.CreateUser(u); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(u)

	case http.MethodPut:
		var u db.User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if err := s.storage.UpdateUser(u); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id", http.StatusBadRequest)
			return
		}
		if err := s.storage.DeleteUser(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type AliasRequest struct {
	UserID string `json:"user_id"`
	Alias  string `json:"alias"`
}

func (s *Server) HandleAddAlias(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		var req AliasRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if err := s.storage.AddUserAlias(req.UserID, req.Alias); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	if r.Method == http.MethodDelete {
		alias := r.URL.Query().Get("alias")
		if alias == "" {
			http.Error(w, "Missing alias", http.StatusBadRequest)
			return
		}
		if err := s.storage.RemoveUserAlias(alias); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type LinkIBANRequest struct {
	UserID            string `json:"user_id"`
	AccountIdentifier string `json:"account_identifier"`
	Note              string `json:"note"`
}

func (s *Server) HandleLinkAccount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		var req LinkIBANRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if err := s.storage.LinkAccountToUser(req.UserID, req.AccountIdentifier, req.Note); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	if r.Method == http.MethodDelete {
		identifier := r.URL.Query().Get("identifier")
		if identifier == "" {
			http.Error(w, "Missing identifier", http.StatusBadRequest)
			return
		}
		if err := s.storage.UnlinkAccountFromUser(identifier); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type UpdateAccountManualRequest struct {
	AccountID   string  `json:"account_id"`
	IBAN        string  `json:"iban"`
	CardNumber  string  `json:"card_number"`
	CreditLimit float64 `json:"credit_limit"`
}

func (s *Server) HandleUpdateAccountManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateAccountManualRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if err := s.storage.UpdateAccountManualDetails(req.AccountID, req.IBAN, req.CardNumber, req.CreditLimit); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
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

func (s *Server) HandleWipeCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.storage.WipeCache(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Кэш выписок очищен. База графа и лиц (core.db) в полной сохранности.",
	})
}
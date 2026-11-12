package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"economic/pkg/binance"
	"economic/pkg/db"
	"economic/pkg/icu"
	"economic/pkg/mono"
	"economic/pkg/privat"
	"economic/pkg/raif"
)

type Server struct {
	storage     *db.Storage
	monoClients []*mono.Client
	raifClients []*raif.Client

	syncMutex  sync.Mutex
	isSyncing  bool
	syncStatus string

	bn binanceState // Binance (read-only), см. handlers_binance.go

	prog progress // структурированный прогресс синка для панели на фронте

	allowedOrigins map[string]bool // CORS: кто из браузера имеет право ходить в API
}

func NewServer(storage *db.Storage, monoClients []*mono.Client, raifClients []*raif.Client) *Server {
	return &Server{
		storage:     storage,
		monoClients: monoClients,
		raifClients: raifClients,
		syncStatus:  "IDLE",
		allowedOrigins: map[string]bool{
			"http://localhost:5173": true,
			"http://127.0.0.1:5173": true,
		},
	}
}

// SetAllowedOrigins задаёт список Origin фронта (например из env ALLOWED_ORIGINS).
func (s *Server) SetAllowedOrigins(origins []string) {
	m := make(map[string]bool, len(origins))
	for _, o := range origins {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			m[o] = true
		}
	}
	if len(m) > 0 {
		s.allowedOrigins = m
	}
}

// oldestTime — самое раннее время в выписке (не полагаемся на порядок, в котором банк отдал операции).
func oldestTime(items []mono.StatementItemDTO) int64 {
	oldest := items[0].Time
	for _, it := range items[1:] {
		if it.Time < oldest {
			oldest = it.Time
		}
	}
	return oldest
}

func (s *Server) updateStatus(msg string) {
	s.syncMutex.Lock()
	s.syncStatus = msg
	s.syncMutex.Unlock()
	log.Printf("[Sync Status] %s", msg)
}

var ruMonthsShort = []string{
	"", "янв.", "февр.", "марта", "апр.", "мая", "июня",
	"июля", "авг.", "сент.", "окт.", "нояб.", "декабря",
}

func formatHumanDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), ruMonthsShort[t.Month()], t.Year())
}

func (s *Server) EnableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Раньше было "*": любая открытая вкладка браузера могла читать финансы и стирать кеш.
		// Запросы без Origin (curl, сам фронт через прокси) пропускаем — бэкенд слушает только localhost.
		if origin := r.Header.Get("Origin"); origin != "" {
			if !s.allowedOrigins[origin] {
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
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
	s.prog.begin()
	defer s.prog.finish()
	defer func() {
		s.syncMutex.Lock()
		s.isSyncing = false
		s.syncStatus = "СНЯТО"
		s.syncMutex.Unlock()
	}()

	if len(s.monoClients) == 0 && len(s.raifClients) == 0 && s.bn.client == nil {
		s.updateStatus("ОТСУТСТВУЮТ КЛЮЧИ API")
		return
	}

	// Все источники опрашиваются параллельно: у каждого ключа свой лимит запросов.
	var wg sync.WaitGroup
	run := func(f func()) {
		wg.Add(1)
		go func() { defer wg.Done(); f() }()
	}
	for i, c := range s.monoClients {
		run(func() { s.crawlBank(s.monoSpec(c, i+1, len(s.monoClients))) })
	}
	for i, c := range s.raifClients {
		run(func() { s.crawlBank(s.raifSpec(c, i+1, len(s.raifClients))) })
	}
	if s.bn.client != nil {
		run(s.crawlBinance)
	}
	wg.Wait()
}

// bankSpec — всё, чем один банковский ключ отличается от другого; сам обход (crawlBank) общий.
type bankSpec struct {
	kind       string // mono | raif — тип клиента в панели прогресса и префикс статусов
	num, total int
	tag        string        // суффикс статусов («RAIF»)
	pause      time.Duration // пауза между запросами к ключу
	retryWait  time.Duration
	pageLimit  int // лимит ответа выписки (0 — неизвестен)
	info       func() (name string, accounts []mono.AccountDTO, err error)
	statement  func(accID string, from, to time.Time) ([]mono.StatementItemDTO, error)
	classify   func(error) errKind
}

func (sp bankSpec) fallbackName() string {
	return fmt.Sprintf("%s %d", strings.ToUpper(sp.kind[:1])+sp.kind[1:], sp.num)
}

func (sp bankSpec) suffix() string {
	if sp.tag == "" {
		return ""
	}
	return " " + sp.tag
}

func (s *Server) monoSpec(client *mono.Client, num, total int) bankSpec {
	return bankSpec{
		kind: "mono", num: num, total: total,
		pause: 61 * time.Second, retryWait: 65 * time.Second, pageLimit: 500, // лимит Monobank на ответ выписки
		info: func() (string, []mono.AccountDTO, error) {
			var info *mono.UserInfoDTO
			var err error
			for attempt := 0; attempt < 3; attempt++ {
				info, err = client.GetClientInfo()
				if _, isRate := err.(*mono.RateLimitError); isRate {
					s.updateStatus(fmt.Sprintf("MONO %d/%d: 429 ЛИМИТ · ОХЛАЖДЕНИЕ 65С", num, total))
					time.Sleep(65 * time.Second)
					continue
				}
				break
			}
			if err != nil {
				return "", nil, err
			}
			return info.Name, info.Accounts, nil
		},
		statement: client.GetStatement,
		classify: func(err error) errKind {
			switch err.(type) {
			case *mono.RateLimitError:
				return errRetry
			case *mono.BadRequestError:
				return errHistoryEnd
			}
			return networkOrFatal(err)
		},
	}
}

func (s *Server) raifSpec(client *raif.Client, num, total int) bankSpec {
	return bankSpec{
		kind: "raif", num: num, total: total, tag: "RAIF",
		pause: 30 * time.Second, retryWait: 30 * time.Second, pageLimit: 0, // лимит ответа Raif неизвестен — считаем осторожно
		info:      client.GetUserInfo,
		statement: client.GetStatement,
		classify:  networkOrFatal,
	}
}

func networkOrFatal(err error) errKind {
	if isNetworkErr(err) {
		return errNetwork
	}
	return errFatal
}

// crawlBank — «Разведать» для одного ключа любого банка: счета → закрытые → выписка каждого счёта окнами.
func (s *Server) crawlBank(sp bankSpec) {
	kindUp := strings.ToUpper(sp.kind)
	ownerName, accounts, err := sp.info()
	if err != nil {
		log.Printf("[Sync Error] %s client %d info: %v", kindUp, sp.num, err)
		s.updateStatus(fmt.Sprintf("%s %d/%d: ОТКАЗ API", kindUp, sp.num, sp.total))
		s.prog.clientError(s.prog.client(sp.fallbackName(), sp.kind, sp.pause), bankErrText(err))
		return
	}
	if ownerName == "" {
		ownerName = sp.fallbackName()
	}
	if err := s.storage.SaveAccounts(accounts, ownerName); err != nil {
		log.Printf("[Sync Error] [%s] Save accounts: %v", ownerName, err)
	}
	if len(accounts) == 0 {
		s.updateStatus(fmt.Sprintf("[%s] СЧЕТА ПУСТЫ%s", ownerName, sp.suffix()))
		return
	}

	cp := s.prog.client(ownerName, sp.kind, sp.pause)
	progs := progAccounts(accounts)
	s.prog.setAccounts(cp, progs)
	ids := make([]string, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.ID)
	}
	if err := s.storage.MarkClosedAbsent(ownerName, ids); err != nil {
		log.Printf("[Sync Error] [%s] closed accounts: %v", ownerName, err)
	}

	var cur *accProg
	coolSleep := func(d time.Duration) { s.prog.cool(cur, d); time.Sleep(d); s.prog.fetching(cur) }
	pace := newPacer(sp.pause, coolSleep)
	for i, acc := range accounts {
		cur = progs[i]
		label := fmt.Sprintf("[%s] СЧЕТ %d/%d (%s)%s", ownerName, i+1, len(accounts), acc.Type, sp.suffix())
		s.updateStatus(label)

		accID := acc.ID
		st, err := s.walkAccount(accID, walkCfg{
			label:     label,
			pageLimit: sp.pageLimit,
			pause:     sp.pause,
			retryWait: sp.retryWait,
			classify:  sp.classify,
			fetch:     func(from, to time.Time) ([]mono.StatementItemDTO, error) { return sp.statement(accID, from, to) },
			sleep:     coolSleep,
			pace:      pace,
			onWindow:  func(f, t int64) { s.prog.window(cur, f, t) },
			now:       time.Now,
			status:    s.updateStatus,
		})
		s.prog.accDone(cur, st, err)
		if errors.Is(err, errOffline) {
			s.abortOffline(progs, i+1)
			break
		}
		if err != nil {
			log.Printf("[Sync Error] [%s] счёт %s: %v", ownerName, acc.ID, err)
			s.updateStatus(fmt.Sprintf("[%s] СБОЙ ВЫПИСКИ%s", acc.Type, sp.suffix()))
			continue
		}
		log.Printf("[Sync] [%s] счёт %s: запросов %d, история целиком: %v", ownerName, acc.ID, st.Requests, st.Complete)
	}
}

func (s *Server) HandleImportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32<<20) // выписка больше 32 МБ — это не выписка
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := "statement"
	if header != nil && header.Filename != "" {
		filename = header.Filename
	}

	rawBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file bytes", http.StatusBadRequest)
		return
	}

	res, err := s.importSource(filename, rawBytes)
	if err != nil {
		code := http.StatusInternalServerError
		if he, ok := err.(*importError); ok {
			code = he.code
		}
		http.Error(w, err.Error(), code)
		return
	}

	// Файл успешно разобран → если он новый (по хэшу содержимого), кладём в кэш источников на диске.
	// Это НЕ кэш-база: переживает любой сброс, из него «Загрузить» восстанавливает данные.
	cached, aerr := archiveSource(res.Card, filename, rawBytes)
	if aerr != nil {
		log.Printf("[Sources] не удалось сохранить источник %q: %v", filename, aerr)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"account_id":     res.AccountID,
		"iban":           res.IBAN,
		"card_number":    res.Card,
		"imported_count": res.Count,
		"source_cached":  cached,
	})
}

type importError struct {
	code int
	msg  string
}

func (e *importError) Error() string { return e.msg }

type importResult struct {
	AccountID, IBAN, Card string
	Count                 int
}

// importSource разбирает выписку (PDF Привата / CSV Райфа / выписка Binance / брокерский отчёт ICU) и пишет её в кеш-базу.
// Общий путь и для загрузки файла в интерфейсе, и для «Загрузить» из кэша источников.
func (s *Server) importSource(filename string, rawBytes []byte) (*importResult, error) {
	var accountID, targetIBAN, targetCard, bankType string
	var balance float64
	var transactions []mono.StatementItemDTO

	// Автоопределение формата: выписка Binance, PDF (Приват) vs CSV (Райф / CSV)
	if binance.IsStatement(rawBytes) {
		return s.importBinanceStatement(filename, rawBytes)
	}
	if icu.IsReport(rawBytes) {
		return s.importICUReport(filename, rawBytes)
	}
	if bytes.HasPrefix(rawBytes, []byte("%PDF")) || strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		parsed, err := privat.ParsePDF(bytes.NewReader(rawBytes))
		if err != nil {
			return nil, &importError{http.StatusBadRequest, fmt.Sprintf("Failed to parse Privat PDF: %v", err)}
		}
		targetIBAN = parsed.IBAN
		targetCard = parsed.CardNumber
		bankType = "privat"
		balance = parsed.Balance
		transactions = parsed.Transactions
	} else {
		parsed, err := raif.ParseCSV(bytes.NewReader(rawBytes))
		if err != nil {
			return nil, &importError{http.StatusBadRequest, fmt.Sprintf("Failed to parse CSV: %v", err)}
		}
		targetIBAN = parsed.IBAN
		targetCard = parsed.CardNumber
		bankType = parsed.BankType
		balance = parsed.Balance
		transactions = parsed.Transactions
	}

	if len(transactions) == 0 {
		return nil, &importError{http.StatusBadRequest, "No transactions found in file"}
	}

	accountID = fmt.Sprintf("%s_%s", bankType, targetIBAN)
	if targetIBAN == "" && targetCard != "" {
		accountID = fmt.Sprintf("%s_%s", bankType, targetCard)
	}

	var maskedPans []string
	if targetCard != "" {
		maskedPans = append(maskedPans, targetCard)
	}

	currency := 980
	if transactions[0].CurrencyCode != 0 {
		currency = transactions[0].CurrencyCode
	}

	accDTO := mono.AccountDTO{
		ID:           accountID,
		IBAN:         targetIBAN,
		MaskedPan:    maskedPans,
		Type:         bankType,
		CurrencyCode: currency,
		Balance:      int64(math.Round(balance * 100)),
	}

	if err := s.storage.SaveImportedAccount(accDTO, filename); err != nil {
		return nil, &importError{http.StatusInternalServerError, fmt.Sprintf("Failed to save account: %v", err)}
	}
	if err := s.storage.SaveTransactions(accountID, transactions); err != nil {
		return nil, &importError{http.StatusInternalServerError, fmt.Sprintf("Failed to save transactions: %v", err)}
	}
	return &importResult{AccountID: accountID, IBAN: targetIBAN, Card: targetCard, Count: len(transactions)}, nil
}

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
		if strings.TrimSpace(u.Name) == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
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
	ParentIdentifier  string `json:"parent_identifier"`
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
		// Note — это название алиаса, к которому относится связь
		if err := s.storage.LinkAccount(req.UserID, req.Note, req.AccountIdentifier, req.ParentIdentifier); err != nil {
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
		// alias обязателен для корректного теневого бана; без него — убрать у всех алиасов
		alias := r.URL.Query().Get("alias")
		if err := s.storage.UnlinkAccount(alias, identifier); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// GET /api/alias-structures — запомненные структуры алиасов (в т.ч. бесхозных, user_id == "").
func (s *Server) HandleAliasStructures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list, err := s.storage.GetAliasStructures()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
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
	accounts, err := s.storage.GetAllAccounts() // счета кеша + ручные (ICU)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accounts)
}

func (s *Server) HandleGetTransactions(w http.ResponseWriter, r *http.Request) {
	limit := 50000
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 500000 {
		limit = v
	}
	txs, err := s.storage.GetAllTransactions(limit) // кеш + записи журналов ручных счетов
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
		"running":  running,
		"status":   statusMsg,
		"stats":    stats,
		"progress": s.progressSnapshot(),
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

type accountLifeRequest struct {
	AccountID string `json:"account_id"`
	Opened    string `json:"opened"` // YYYY-MM-DD или "" (по выписке)
	Closed    string `json:"closed"` // YYYY-MM-DD или "" (не закрыт / по данным банка)
}

// HandleSetAccountLife — ручные даты открытия/закрытия счёта (банк их не отдаёт).
func (s *Server) HandleSetAccountLife(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req accountLifeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AccountID == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
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
		http.Error(w, "Некорректные даты", http.StatusBadRequest)
		return
	}
	if err := s.storage.SetAccountLife(req.AccountID, opened, closed); err != nil {
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

// bankErrText — текст ошибки для панели: сеть и отказ банка различаем.
func bankErrText(err error) string {
	if isNetworkErr(err) {
		return errOffline.Error()
	}
	return "банк отказал: " + err.Error()
}

// abortOffline — сеть пропала: остальные счета ключа помечаем ошибкой и не опрашиваем.
func (s *Server) abortOffline(progs []*accProg, from int) {
	for j := from; j < len(progs); j++ {
		s.prog.accDone(progs[j], walkStats{}, errOffline)
	}
}

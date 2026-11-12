package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"economic/pkg/db"
	"economic/pkg/mono"
	"economic/pkg/raif"
)

type Server struct {
	storage     *db.Storage
	monoClients []*mono.Client
	raifClients []*raif.Client

	syncMutex  sync.Mutex
	isSyncing  bool
	syncCancel context.CancelFunc // отменяет идущий «Разведать» (сброс кеша)
	syncDone   chan struct{}      // закрывается, когда обход действительно остановился
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

// beginSync помечает «Разведать» запущенным; false — он уже идёт. ctx отменяется сбросом кеша.
func (s *Server) beginSync() (ctx context.Context, done chan struct{}, ok bool) {
	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()
	if s.isSyncing {
		return nil, nil, false
	}
	ctx, s.syncCancel = context.WithCancel(context.Background())
	s.isSyncing, s.syncStatus, s.syncDone = true, "Инициализация API...", make(chan struct{})
	return ctx, s.syncDone, true
}

// abortSync останавливает идущий «Разведать» и ждёт, пока он отпустит кеш (не дольше нескольких секунд).
// Остановленный обход сам ничего не пишет в кеш и не трогает состояние: оно уже сброшено здесь.
func (s *Server) abortSync() {
	s.syncMutex.Lock()
	cancel, done, was := s.syncCancel, s.syncDone, s.isSyncing
	if was {
		s.isSyncing, s.syncStatus, s.syncCancel = false, "СНЯТО (КЕШ СБРОШЕН)", nil
	}
	s.syncMutex.Unlock()
	if !was {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
	s.prog.reset()
}

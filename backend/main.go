package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"economic/pkg/api"
	"economic/pkg/binance"
	"economic/pkg/db"
	"economic/pkg/mono"
	"economic/pkg/raif"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("INFO: .env file not found, using environment variables")
	}

	// 1. Токены Monobank (MONO_TOKENS или MONO_TOKEN)
	tokensStr := os.Getenv("MONO_TOKENS")
	if tokensStr == "" {
		tokensStr = os.Getenv("MONO_TOKEN")
	}

	var monoClients []*mono.Client
	if tokensStr != "" {
		for _, t := range strings.Split(tokensStr, ",") {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				monoClients = append(monoClients, mono.NewClient(trimmed))
			}
		}
	}

	if len(monoClients) == 0 {
		log.Println("WARNING: No Monobank tokens provided!")
	} else {
		log.Printf("INFO: Loaded %d Monobank API key(s)", len(monoClients))
	}

	// 2. Токены Raiffeisen Bank (RAIF_TOKENS или RAIF_TOKEN)
	raifTokensStr := os.Getenv("RAIF_TOKENS")
	if raifTokensStr == "" {
		raifTokensStr = os.Getenv("RAIF_TOKEN")
	}

	var raifClients []*raif.Client
	if raifTokensStr != "" {
		for _, t := range strings.Split(raifTokensStr, ",") {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				raifClients = append(raifClients, raif.NewClient(trimmed))
			}
		}
	}

	if len(raifClients) == 0 {
		log.Println("INFO: No Raiffeisen Bank tokens provided")
	} else {
		log.Printf("INFO: Loaded %d Raiffeisen Bank API key(s)", len(raifClients))
	}

	// 3. Binance: выписка (CSV) + публичные минутные цены, ключей нет
	binanceClient := binance.NewClient()

	cacheDBPath := os.Getenv("CACHE_DB_PATH")
	if cacheDBPath == "" {
		cacheDBPath = "cache.db"
	}

	coreDBPath := os.Getenv("CORE_DB_PATH")
	if coreDBPath == "" {
		coreDBPath = "core.db"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	storage, err := db.NewStorage(cacheDBPath, coreDBPath)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}

	server := api.NewServer(storage, monoClients, raifClients)
	server.SetBinance(binanceClient)
	if o := os.Getenv("ALLOWED_ORIGINS"); o != "" {
		server.SetAllowedOrigins(strings.Split(o, ","))
	}

	// API Роуты Счетов и Транзакций (Cache DB)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/accounts", server.EnableCORS(server.HandleGetAccounts))
	mux.HandleFunc("/api/accounts/life", server.EnableCORS(server.HandleSetAccountLife))
	mux.HandleFunc("/api/accounts/update", server.EnableCORS(server.HandleUpdateAccountManual))
	mux.HandleFunc("/api/accounts/credit-limit", server.EnableCORS(server.HandleUpdateCreditLimit))
	mux.HandleFunc("/api/transactions", server.EnableCORS(server.HandleGetTransactions))

	// Брокерский счёт ICU: журнал из отчёта (только чтение) и справочник облигаций
	mux.HandleFunc("/api/icu/ledger", server.EnableCORS(server.HandleICULedger))
	mux.HandleFunc("/api/icu/bonds", server.EnableCORS(server.HandleICUBonds))

	// Binance (только чтение кеша; обновляется через «Разведать»)
	mux.HandleFunc("/api/binance", server.EnableCORS(server.HandleBinance))
	mux.HandleFunc("/api/binance/candles", server.EnableCORS(server.HandleBinanceCandles)) // свечи оценки портфеля для графика
	mux.HandleFunc("/api/binance/moves", server.EnableCORS(server.HandleBinanceMoves))     // переводы между кошельками для истории
	mux.HandleFunc("/api/binance/trades", server.EnableCORS(server.HandleBinanceTrades))   // покупки и продажи монет для истории

	// API Роуты Синхронизации и Очистки Кэша
	mux.HandleFunc("/api/sync/status", server.EnableCORS(server.HandleGetStatus))
	mux.HandleFunc("/api/sync/full", server.EnableCORS(server.HandleFullHistoricalSync))
	mux.HandleFunc("/api/sync/wipe-cache", server.EnableCORS(server.HandleWipeCache))
	mux.HandleFunc("/api/sources/load", server.EnableCORS(server.HandleLoadSources))

	// API Роуты Сущностей и Связей (Core DB: Лица / IBAN / Алиасы)
	mux.HandleFunc("/api/users", server.EnableCORS(server.HandleUsers))
	mux.HandleFunc("/api/users/alias", server.EnableCORS(server.HandleAddAlias))
	mux.HandleFunc("/api/users/link-account", server.EnableCORS(server.HandleLinkAccount))
	mux.HandleFunc("/api/alias-structures", server.EnableCORS(server.HandleAliasStructures))

	mux.HandleFunc("/api/sync/import-csv", server.EnableCORS(server.HandleImportCSV))

	// По умолчанию слушаем только localhost: это личные финансы, а не публичный сервис.
	// Для доступа с другого устройства: HOST=0.0.0.0 (и добавь его Origin в ALLOWED_ORIGINS).
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	srv := &http.Server{
		Addr:              host + ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute, // загрузка выписки
		IdleTimeout:       2 * time.Minute,
	}

	log.Printf("Backend running on http://%s", srv.Addr)
	log.Printf("Storage Architecture: Cache DB [%s] | Core DB [%s]", cacheDBPath, coreDBPath)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Ctrl+C / SIGTERM: дожидаемся текущих запросов и закрываем базы (сбрасывается WAL).
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	if err := storage.Close(); err != nil {
		log.Printf("storage close: %v", err)
	}
}

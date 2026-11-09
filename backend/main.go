package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"economic/pkg/api"
	"economic/pkg/db"
	"economic/pkg/mono"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("INFO: .env file not found, using environment variables")
	}

	monoToken := os.Getenv("MONO_TOKEN")
	if monoToken == "" {
		log.Println("WARNING: MONO_TOKEN is empty! Mono sync will fail.")
	}

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

	monoClient := mono.NewClient(monoToken)
	server := api.NewServer(storage, monoClient)

	// API Роуты Счетов и Транзакций (Cache DB)
	http.HandleFunc("/api/accounts", server.EnableCORS(server.HandleGetAccounts))
	http.HandleFunc("/api/accounts/update", server.EnableCORS(server.HandleUpdateAccountManual))
	http.HandleFunc("/api/accounts/credit-limit", server.EnableCORS(server.HandleUpdateCreditLimit))
	http.HandleFunc("/api/transactions", server.EnableCORS(server.HandleGetTransactions))
	
	// API Роуты Синхронизации и Очистки Кэша
	http.HandleFunc("/api/sync/status", server.EnableCORS(server.HandleGetStatus))
	http.HandleFunc("/api/sync/full", server.EnableCORS(server.HandleFullHistoricalSync))
	http.HandleFunc("/api/sync/wipe-cache", server.EnableCORS(server.HandleWipeCache))

	// API Роуты Сущностей и Связей (Core DB: Лица / IBAN / Алиасы)
	http.HandleFunc("/api/users", server.EnableCORS(server.HandleUsers))
	http.HandleFunc("/api/users/alias", server.EnableCORS(server.HandleAddAlias))
	http.HandleFunc("/api/users/link-account", server.EnableCORS(server.HandleLinkAccount))

	log.Printf("Backend running on http://localhost:%s", port)
	log.Printf("Storage Architecture: Cache DB [%s] | Core DB [%s]", cacheDBPath, coreDBPath)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
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

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "finance.db"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	storage, err := db.NewStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}

	monoClient := mono.NewClient(monoToken)
	server := api.NewServer(storage, monoClient)

	http.HandleFunc("/api/accounts", server.EnableCORS(server.HandleGetAccounts))
	http.HandleFunc("/api/transactions", server.EnableCORS(server.HandleGetTransactions))
	http.HandleFunc("/api/sync/status", server.EnableCORS(server.HandleGetStatus))
	http.HandleFunc("/api/sync/full", server.EnableCORS(server.HandleFullHistoricalSync))
	http.HandleFunc("/api/accounts/credit-limit", server.EnableCORS(server.HandleUpdateCreditLimit))

	log.Printf("Backend server running on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
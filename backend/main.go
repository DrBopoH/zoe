package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"economic/pkg/api"
	"economic/pkg/binance"
	"economic/pkg/db"
	"economic/pkg/market"
	"economic/pkg/mono"
	"economic/pkg/raif"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("INFO: .env file not found, using environment variables")
	}

	// Токены банков: MONO_TOKENS / MONO_TOKEN, RAIF_TOKENS / RAIF_TOKEN (через запятую)
	monoClients := api.ClientsFromEnv(mono.NewClient, "MONO_TOKENS", "MONO_TOKEN")
	if len(monoClients) == 0 {
		log.Println("WARNING: No Monobank tokens provided!")
	} else {
		log.Printf("INFO: Loaded %d Monobank API key(s)", len(monoClients))
	}

	raifClients := api.ClientsFromEnv(func(t string) *raif.Client { return raif.NewClient(t) }, "RAIF_TOKENS", "RAIF_TOKEN")
	if len(raifClients) == 0 {
		log.Println("INFO: No Raiffeisen Bank tokens provided")
	} else {
		log.Printf("INFO: Loaded %d Raiffeisen Bank API key(s)", len(raifClients))
	}

	cacheDBPath := api.EnvOr("CACHE_DB_PATH", "cache.db")
	coreDBPath := api.EnvOr("CORE_DB_PATH", "core.db")
	port := api.EnvOr("PORT", "8080")

	storage, err := db.NewStorage(cacheDBPath, coreDBPath)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}

	server := api.NewServer(storage, monoClients, raifClients)
	// Binance: выписка (CSV) + публичные минутные цены, ключей нет
	server.SetBinance(binance.NewClient())
	// Облигации: ОВДП и US Treasury из публичных API, обновление только по запросу
	marketClient := market.NewClient()
	// Размер сделки P2P: объявления, по которым нельзя провести сделку хотя бы на эту сумму, цену P2P не задают (по умолчанию 1000 ₴)
	if v := os.Getenv("P2P_MIN_DEAL_UAH"); v != "" {
		if n, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil && n > 0 {
			marketClient.P2PMinDeal = n
		} else {
			log.Printf("WARNING: P2P_MIN_DEAL_UAH=%q не разобрать (пример: 1000, 5000), используется 1000", v)
		}
	}
	server.SetMarket(marketClient)
	// Кеш макро-рядов, если он пуст (первый запуск после «стереть кеш» или переезда), собирается из файлов группы «Экономика»
	if n, err := server.RestoreEconomy(); err != nil {
		log.Printf("WARNING: кеш рядов из файлов не собран: %v", err)
	} else if n > 0 {
		log.Printf("INFO: из файлов «Экономики» в кеш добавлено рядов: %d", n)
	}
	// Кеш акций (цены, сплиты, число акций из отчётности SEC) — тоже из файлов «Экономики», если его в кеше нет: заново это около 6 минут запросов
	if t, f, err := server.RestoreStocks(); err != nil {
		log.Printf("WARNING: кеш акций из файлов не собран: %v", err)
	} else if t+f > 0 {
		log.Printf("INFO: из файлов «Экономики» в кеш акций добавлено: тикеров %d, кадров отчётности %d", t, f)
	}
	// Снимки крипто-контура (P2P-цена, комиссии сетей, доминация) копятся сами раз в MARKET_EVERY (по умолчанию час; 0 — выключено)
	every := market.TickEvery
	if v := os.Getenv("MARKET_EVERY"); v != "" {
		if v == "0" {
			every = 0
		} else if d, err := time.ParseDuration(v); err == nil && d > 0 {
			every = d
		} else {
			log.Printf("WARNING: MARKET_EVERY=%q не разобрать (пример: 1h, 30m, 0), используется %s", v, every)
		}
	}
	scheduleCtx, stopSchedule := context.WithCancel(context.Background())
	defer stopSchedule()
	go server.RunMarketSchedule(scheduleCtx, every)
	if o := os.Getenv("ALLOWED_ORIGINS"); o != "" {
		server.SetAllowedOrigins(strings.Split(o, ","))
	}

	// По умолчанию слушаем только localhost: это личные финансы, а не публичный сервис.
	// Для доступа с другого устройства: HOST=0.0.0.0 (и добавь его Origin в ALLOWED_ORIGINS).
	host := api.EnvOr("HOST", "127.0.0.1")
	srv := &http.Server{
		Addr:              host + ":" + port,
		Handler:           server.Handler(),
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

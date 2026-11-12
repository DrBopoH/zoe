// Package api — HTTP-слой бэкенда. Маршруты собраны в routes.go; вспомогательные функции ответов — в httpx.go.
//
//	server.go            Server, CORS, запуск/остановка синхронизации
//	crawler.go           «Разведать»: обход банков (mono, raif)
//	handlers_accounts.go счета, транзакции, статус, сброс кеша
//	handlers_users.go    лица, алиасы, связи, скрытые виды IBAN
//	handlers_import.go   импорт выписки (CSV/PDF)
//	handlers_binance.go  Binance (чтение кеша, свечи)
//	handlers_icu.go      брокерский счёт ICU
//	sources*.go          файлы-источники (.zoe)
package api

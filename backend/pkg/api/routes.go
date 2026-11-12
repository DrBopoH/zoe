package api

import "net/http"

// Handler — все маршруты API; CORS навешивается один раз на всё, а не на каждый маршрут отдельно
// (новый маршрут нельзя «забыть» защитить).
func (s *Server) Handler() http.Handler {
	routes := []struct {
		path string
		h    http.HandlerFunc
	}{
		// Счета и транзакции (Cache DB)
		{"/api/accounts", s.HandleGetAccounts},
		{"/api/accounts/life", s.HandleSetAccountLife},
		{"/api/accounts/update", s.HandleUpdateAccountManual},
		{"/api/accounts/credit-limit", s.HandleUpdateCreditLimit},
		{"/api/transactions", s.HandleGetTransactions},

		// Брокерский счёт ICU: журнал из отчёта (только чтение) и справочник облигаций
		{"/api/icu/ledger", s.HandleICULedger},
		{"/api/icu/bonds", s.HandleICUBonds},

		// Рыночные облигации: ОВДП (НБУ) и US Treasury, публичные данные в кеше
		{"/api/market/bonds", s.HandleMarketBonds},
		{"/api/market/sync", s.HandleMarketSync},
		{"/api/market/lists", s.HandleMarketLists}, // свои списки ISIN (например «icu» — доступно к покупке в брокере)
		{"/api/market/curve", s.HandleMarketCurve}, // кривая доходности рынка (точки срок → %) для оценки бумаг без цены
		{"/api/macro", s.HandleMacro},              // ставки НБУ и инфляция (ряды из кеша)

		// Binance (только чтение кеша; обновляется через «Разведать»)
		{"/api/binance", s.HandleBinance},
		{"/api/binance/candles", s.HandleBinanceCandles}, // свечи оценки портфеля для графика
		{"/api/binance/moves", s.HandleBinanceMoves},     // переводы между кошельками для истории
		{"/api/binance/trades", s.HandleBinanceTrades},   // покупки и продажи монет для истории
		{"/api/binance/swaps", s.HandleBinanceSwaps},     // Convert, пыль → BNB, стейкинг для истории

		// Синхронизация, импорт и очистка кеша
		{"/api/sync/status", s.HandleGetStatus},
		{"/api/sync/full", s.HandleFullHistoricalSync},
		{"/api/sync/wipe-cache", s.HandleWipeCache},
		{"/api/sync/import-csv", s.HandleImportCSV},

		// Файлы-источники (.zoe)
		{"/api/sources", s.HandleSources},
		{"/api/sources/load", s.HandleLoadSources},
		{"/api/sources/reset", s.HandleResetSources},
		{"/api/sources/inspect", s.HandleInspectSource},
		{"/api/sources/save-api", s.HandleSaveAPI},
		{"/api/sources/file", s.HandleDeleteSource},

		// Сущности и связи (Core DB: Лица / IBAN / Алиасы)
		{"/api/users", s.HandleUsers},
		{"/api/users/alias", s.HandleAddAlias},
		{"/api/users/link-account", s.HandleLinkAccount},
		{"/api/alias-structures", s.HandleAliasStructures},
		{"/api/iban-masks", s.HandleIBANMasks},
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.HandleFunc(r.path, r.h)
	}
	return s.EnableCORS(mux.ServeHTTP)
}

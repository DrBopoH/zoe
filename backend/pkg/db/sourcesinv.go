package db

import (
	"math"

	"economic/pkg/mono"
)

// Что лежит в кеше — для окна «Выписки»: сколько записей у каждого счёта и точечный сброс.

// CacheCounts — число записей кеша по счетам: операции банков, журнал брокера, журнал Binance.
func (s *Storage) CacheCounts() (map[string]int, error) {
	out := map[string]int{}
	for _, q := range []string{
		`SELECT account_id, COUNT(*) FROM transactions GROUP BY account_id`,
		`SELECT account_id, COUNT(*) FROM icu_flows GROUP BY account_id`,
	} {
		rows, err := s.cacheDB.Query(q)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var n int
			if err := rows.Scan(&id, &n); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] += n
		}
		rows.Close()
	}
	for _, t := range []string{"binance_ledger", "binance_p2p"} { // журнал выписки и история C2C-ордеров
		var n int
		if err := s.cacheDB.QueryRow(`SELECT COUNT(*) FROM ` + t).Scan(&n); err == nil && n > 0 {
			out[BinanceAccountID] += n
		}
	}
	return out, nil
}

// WipeAccounts стирает из кеша данные выбранных счетов (core.db — алиасы, лица — не трогается).
func (s *Storage) WipeAccounts(ids []string) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		for _, q := range []string{
			`DELETE FROM transactions WHERE account_id=?`,
			`DELETE FROM sync_coverage WHERE account_id=?`,
			`DELETE FROM icu_flows WHERE account_id=?`,
			`DELETE FROM accounts WHERE id=?`,
		} {
			if _, err := tx.Exec(q, id); err != nil {
				return err
			}
		}
		if id == BinanceAccountID {
			for _, t := range []string{"binance_ledger", "binance_p2p", "binance_minutes", "binance_state"} {
				if _, err := tx.Exec(`DELETE FROM ` + t); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

// AccountOwner — алиас владельца счёта (как в имени файла/пользователя API).
func (s *Storage) AccountOwner(id string) string {
	var o string
	_ = s.cacheDB.QueryRow(`SELECT COALESCE(owner,'') FROM accounts WHERE id=?`, id).Scan(&o)
	return o
}

// AccountTxs — все операции счёта из кеша в том виде, в каком их отдаёт банк (копейки): для файла-источника.
func (s *Storage) AccountTxs(id string, idPrefix string) ([]mono.StatementItemDTO, error) {
	rows, err := s.cacheDB.Query(`SELECT id, time, COALESCE(description,''), COALESCE(comment,''), COALESCE(counter_edrpou,''),
		COALESCE(counter_iban,''), COALESCE(counter_name,''), COALESCE(mcc,0), COALESCE(original_mcc,0), COALESCE(amount,0),
		COALESCE(operation_amount,0), COALESCE(currency_code,0), COALESCE(commission_rate,0), COALESCE(cashback_amount,0),
		COALESCE(is_hold,0) FROM transactions WHERE account_id=? AND id LIKE ? ORDER BY time, id`, id, idPrefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cents := func(v float64) int64 { return int64(math.Round(v * 100)) }
	var out []mono.StatementItemDTO
	for rows.Next() {
		var t mono.StatementItemDTO
		var amount, opAmount, comm, cash float64
		if err := rows.Scan(&t.ID, &t.Time, &t.Description, &t.Comment, &t.CounterEdrpou, &t.CounterIban, &t.CounterName, &t.MCC,
			&t.OriginalMCC, &amount, &opAmount, &t.CurrencyCode, &comm, &cash, &t.IsHold); err != nil {
			return nil, err
		}
		t.Amount, t.OperationAmount, t.CommissionRate, t.CashbackAmount = cents(amount), cents(opAmount), cents(comm), cents(cash)
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClearCoverage забывает, что счёт уже снят с API: следующая синхронизация снимет его историю заново.
func (s *Storage) ClearCoverage(id string) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()
	_, err := s.cacheDB.Exec(`DELETE FROM sync_coverage WHERE account_id=?`, id)
	return err
}

// BinanceSeriesDump — минутный ряд целиком: первая минута, значения подряд (дыры закрыты предыдущим значением), «до начала», баланс.
func (s *Storage) BinanceSeriesDump() (start int64, vals []float64, origin float64, hasOrigin bool, portfolio float64, ok bool) {
	rows, err := s.cacheDB.Query(`SELECT ts, v FROM binance_minutes ORDER BY ts`)
	if err != nil {
		return
	}
	defer rows.Close()
	var prevTS int64
	for rows.Next() {
		var ts int64
		var v float64
		if rows.Scan(&ts, &v) != nil {
			return 0, nil, 0, false, 0, false
		}
		if len(vals) == 0 {
			start = ts
		} else {
			for t := prevTS + 60; t < ts; t += 60 {
				vals = append(vals, vals[len(vals)-1])
			}
		}
		vals = append(vals, math.Round(v*1e4)/1e4)
		prevTS = ts
	}
	if len(vals) == 0 {
		return
	}
	origin, hasOrigin = s.binanceOrigin()
	_ = s.cacheDB.QueryRow(`SELECT balance FROM accounts WHERE id=?`, BinanceAccountID).Scan(&portfolio)
	return start, vals, origin, hasOrigin, portfolio, true
}

// AccountTxIDs — id операций счёта в кеше за отрезок [from, to] (unix-секунды).
func (s *Storage) AccountTxIDs(id string, from, to int64) (map[string]bool, error) {
	rows, err := s.cacheDB.Query(`SELECT id FROM transactions WHERE account_id=? AND time>=? AND time<=?`, id, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out[t] = true
	}
	return out, rows.Err()
}

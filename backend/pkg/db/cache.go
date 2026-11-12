package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"economic/pkg/mono"
)

// ───────────────────────── запись в кеш ─────────────────────────

// SaveAccounts — счета, полученные из банковского API (источник истины:
// баланс обновляется, первый PAN из ответа банка становится главным).
func (s *Storage) SaveAccounts(accounts []mono.AccountDTO, ownerName string) error {
	return s.saveAccounts(accounts, ownerName, "api")
}

// SaveImportedAccount — счёт из файла выписки (CSV/PDF). Не перетирает
// главный PAN и баланс «живых» данных; баланс считается суммой операций.
func (s *Storage) SaveImportedAccount(acc mono.AccountDTO, ownerName string) error {
	return s.saveAccounts([]mono.AccountDTO{acc}, ownerName, "import")
}

func (s *Storage) saveAccounts(accounts []mono.AccountDTO, ownerName, source string) error {
	if err := s.writeAccounts(accounts, ownerName, source); err != nil {
		return err
	}
	s.syncAliases(nil)
	return nil
}

func (s *Storage) writeAccounts(accounts []mono.AccountDTO, ownerName, source string) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()

	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, acc := range accounts {
		var exIBAN, exMain, exExtra string
		err := tx.QueryRow(`SELECT COALESCE(iban,''), COALESCE(masked_pan,''), COALESCE(extra_pans,'') FROM accounts WHERE id=?`, acc.ID).
			Scan(&exIBAN, &exMain, &exExtra)
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		incoming := mergePans(acc.MaskedPan)
		existing := mergePans([]string{exMain}, splitPans(exExtra))
		var pans []string
		if source == "api" {
			pans = mergePans(incoming, existing) // банк сказал, какая карта главная
		} else {
			pans = mergePans(existing, incoming) // файл не меняет главную карту
		}
		main, extra := "", ""
		if len(pans) > 0 {
			main = pans[0]
			extra = strings.Join(pans[1:], ", ")
		}

		iban := acc.IBAN
		if iban == "" {
			iban = exIBAN
		}
		name := acc.Type
		if ownerName != "" {
			name = fmt.Sprintf("%s (%s)", ownerName, acc.Type)
		}

		_, err = tx.Exec(`
			INSERT INTO accounts (id, name, balance, credit_limit, type, currency_code, iban, masked_pan, extra_pans, source, owner)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET
				name = excluded.name,
				type = excluded.type,
				currency_code = excluded.currency_code,
				iban = excluded.iban,
				masked_pan = excluded.masked_pan,
				extra_pans = excluded.extra_pans,
				source = excluded.source,
				owner = CASE WHEN excluded.source = 'api' THEN excluded.owner ELSE accounts.owner END,
				closed_at = CASE WHEN excluded.source = 'api' THEN 0 ELSE accounts.closed_at END,
				balance = CASE WHEN excluded.source = 'api' THEN excluded.balance ELSE accounts.balance END,
				credit_limit = CASE WHEN excluded.credit_limit > 0 THEN excluded.credit_limit ELSE accounts.credit_limit END`,
			acc.ID, name, float64(acc.Balance)/100.0, float64(acc.CreditLimit)/100.0,
			acc.Type, acc.CurrencyCode, iban, main, extra, source, ownerName)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) SaveTransactions(accID string, items []mono.StatementItemDTO) error {
	if err := s.writeTransactions(accID, items); err != nil {
		return err
	}
	s.syncAliases(nil)
	return nil
}

func (s *Storage) writeTransactions(accID string, items []mono.StatementItemDTO) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	defer s.bump()

	tx, err := s.cacheDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO transactions (
			id, account_id, time, description, comment, counter_edrpou, counter_iban,
			counter_name, mcc, original_mcc, amount, operation_amount, currency_code,
			commission_rate, cashback_amount, is_hold
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, it := range items {
		if _, err := stmt.Exec(
			it.ID, accID, it.Time, it.Description, it.Comment, it.CounterEdrpou, it.CounterIban,
			it.CounterName, it.MCC, it.OriginalMCC,
			float64(it.Amount)/100.0, float64(it.OperationAmount)/100.0, it.CurrencyCode,
			float64(it.CommissionRate)/100.0, float64(it.CashbackAmount)/100.0, it.IsHold,
		); err != nil {
			return err
		}
	}

	// Баланс как сумма операций — только для счетов из файлов выписок.
	// У счетов из банковского API баланс настоящий, его не трогаем.
	if _, err := tx.Exec(`
		UPDATE accounts
		SET balance = (SELECT COALESCE(SUM(amount),0) FROM transactions WHERE account_id = ?)
		WHERE id = ? AND source = 'import'`, accID, accID); err != nil {
		return err
	}
	return tx.Commit()
}

// ───────────────────────── чтение кеша ─────────────────────────

func (s *Storage) GetAccounts() ([]Account, error) {
	rows, err := s.cacheDB.Query(`
		SELECT id, COALESCE(name,''), COALESCE(balance,0), COALESCE(credit_limit,0),
		       COALESCE(type,''), COALESCE(currency_code,0), COALESCE(iban,''),
		       COALESCE(masked_pan,''), COALESCE(extra_pans,''), COALESCE(source,'api'),
		       COALESCE(opened_at,0), COALESCE(closed_at,0),
		       COALESCE((SELECT MIN(time) FROM transactions WHERE account_id = accounts.id),0)
		FROM accounts ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	var list []Account
	for rows.Next() {
		var a Account
		var main, extra string
		var firstTx int64
		if err := rows.Scan(&a.ID, &a.Name, &a.Balance, &a.CreditLimit, &a.Type, &a.Currency,
			&a.IBAN, &main, &extra, &a.Source, &a.OpenedAt, &a.ClosedAt, &firstTx); err != nil {
			rows.Close()
			return nil, err
		}
		// Файл выписки и есть вся известная история: открытие = первая операция в нём.
		if a.OpenedAt == 0 && a.Source == "import" {
			a.OpenedAt = firstTx
		}
		a.IsClosed = a.ClosedAt > 0
		if a.Type == BinanceAccountType || a.Type == ICUAccountType { // источник со своей модалкой, а не банк с выпиской
			a.BankKey, a.HasDetail = a.Type, true
		}
		a.Pans = mergePans([]string{main}, splitPans(extra))
		list = append(list, a)
	}
	rows.Close()

	ov := s.loadOverrides()
	for i := range list {
		a := &list[i]
		if o, ok := ov[a.ID]; ok {
			if o.iban != "" {
				a.IBAN = o.iban
			}
			if o.pan != "" {
				a.Pans = mergePans([]string{o.pan}, a.Pans)
			}
			if o.credit.Valid {
				a.CreditLimit = o.credit.Float64
			}
			if o.opened.Valid {
				a.OpenedAt, a.ManualOpened = o.opened.Int64, true
			}
			if o.closed.Valid {
				a.ClosedAt, a.IsClosed, a.ManualClosed = o.closed.Int64, true, true
			}
		}
		if len(a.Pans) > 0 {
			a.MaskedPan = a.Pans[0]
		}
		if a.Pans == nil {
			a.Pans = []string{}
		}
	}
	return list, nil
}

func (s *Storage) GetTransactions(limit int) ([]Transaction, error) {
	rows, err := s.cacheDB.Query(`
		SELECT id, account_id, time, COALESCE(description,''), COALESCE(comment,''),
		       COALESCE(counter_iban,''), COALESCE(counter_name,''), COALESCE(mcc,0),
		       COALESCE(amount,0), COALESCE(currency_code,0),
		       COALESCE(operation_amount,0), COALESCE(commission_rate,0), COALESCE(cashback_amount,0), COALESCE(is_hold,0)
		FROM transactions ORDER BY time DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := s.buildResolver()
	own := s.buildOwnMatcher()
	list := []Transaction{}
	for rows.Next() {
		var t Transaction
		var ts int64
		var hold int
		if err := rows.Scan(&t.ID, &t.AccountID, &ts, &t.Description, &t.Comment, &t.CounterIBAN,
			&t.CounterName, &t.MCC, &t.Amount, &t.CurrencyCode,
			&t.OperationAmount, &t.Commission, &t.Cashback, &hold); err != nil {
			return nil, err
		}
		t.IsHold = hold != 0
		t.Time = time.Unix(ts, 0).Format(time.RFC3339)
		t.ResolvedUserID = res.resolve(t.CounterIBAN, t.CounterName, t.Description)
		t.OwnAccountID = own.own(t.AccountID, t.CounterIBAN, t.CounterName, t.Description)
		if t.CounterIBAN == "" {
			t.AliasAccts = res.aliasAccts(t.CounterIBAN, t.CounterName, t.Description)
		}
		if t.OwnAccountID == "" { // номера в операции нет: свой счёт по привязке алиаса, если среди его счетов ровно один наш (другой, чем у операции)
			t.OwnAccountID = own.fromAliasAccts(t.AccountID, t.AliasAccts)
		}
		if t.ResolvedUserID == "" && t.OwnAccountID != "" { // контрагент назван номером своей карты: Лицо — владелец этого счёта
			for _, id := range own.ids[t.OwnAccountID] {
				if u := res.resolve(id, "", ""); u != "" {
					t.ResolvedUserID = u
					break
				}
			}
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (s *Storage) GetSyncStats() (map[string]interface{}, error) {
	var total int
	_ = s.cacheDB.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&total)

	var oldest, newest sql.NullInt64
	_ = s.cacheDB.QueryRow(`SELECT MIN(time), MAX(time) FROM transactions`).Scan(&oldest, &newest)

	o, n := "—", "—"
	if oldest.Valid {
		o = time.Unix(oldest.Int64, 0).Format("02.01.2006")
	}
	if newest.Valid {
		n = time.Unix(newest.Int64, 0).Format("02.01.2006")
	}
	return map[string]interface{}{
		"total_transactions": total, "oldest_tx_time": o, "newest_tx_time": n,
		"data_version": s.Version(),
	}, nil
}

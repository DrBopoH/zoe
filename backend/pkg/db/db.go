package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
	"economic/pkg/mono"
)

type Storage struct {
	db *sql.DB
}

func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		currency_code INTEGER NOT NULL,
		balance INTEGER NOT NULL,
		credit_limit INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS transactions (
		id TEXT PRIMARY KEY,
		account_id TEXT NOT NULL,
		time DATETIME NOT NULL,
		description TEXT NOT NULL,
		mcc INTEGER NOT NULL,
		original_mcc INTEGER NOT NULL,
		amount INTEGER NOT NULL,
		operation_amount INTEGER NOT NULL,
		currency_code INTEGER NOT NULL,
		commission_rate INTEGER DEFAULT 0,
		cashback_amount INTEGER DEFAULT 0,
		is_hold BOOLEAN DEFAULT FALSE,
		FOREIGN KEY(account_id) REFERENCES accounts(id)
	);`

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	return &Storage{db: db}, nil
}

func (s *Storage) SaveAccounts(accounts []mono.AccountDTO, ownerName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// ON CONFLICT обновляет только баланс, оставляя пользовательский credit_limit
	stmt, err := tx.Prepare(`
		INSERT INTO accounts (id, name, type, currency_code, balance, credit_limit, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			balance=excluded.balance,
			updated_at=excluded.updated_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, acc := range accounts {
		name := fmt.Sprintf("%s (%s)", ownerName, acc.Type)
		if len(acc.MaskedPan) > 0 {
			name = fmt.Sprintf("%s *%s", acc.Type, acc.MaskedPan[0][len(acc.MaskedPan[0])-4:])
		}
		_, err := stmt.Exec(acc.ID, name, acc.Type, acc.CurrencyCode, acc.Balance, acc.CreditLimit, time.Now())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Storage) UpdateCreditLimit(accountID string, creditLimit float64) error {
	limitInt := int64(creditLimit * 100) // переводим гривны в копейки
	_, err := s.db.Exec("UPDATE accounts SET credit_limit = ? WHERE id = ?", limitInt, accountID)
	return err
}

func (s *Storage) SaveTransactions(accountID string, items []mono.StatementItemDTO) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO transactions (id, account_id, time, description, mcc, original_mcc, amount, operation_amount, currency_code, commission_rate, cashback_amount, is_hold)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			is_hold=excluded.is_hold,
			amount=excluded.amount
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		t := time.Unix(item.Time, 0)
		_, err := stmt.Exec(
			item.ID, accountID, t, item.Description, item.MCC, item.OriginalMCC,
			item.Amount, item.OperationAmount, item.CurrencyCode,
			item.CommissionRate, item.CashbackAmount, item.IsHold,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

type AccountResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	CurrencyCode int     `json:"currency_code"`
	Balance      float64 `json:"balance"`
	CreditLimit  float64 `json:"credit_limit"`
}

func (s *Storage) GetAccounts() ([]AccountResponse, error) {
	rows, err := s.db.Query("SELECT id, name, type, currency_code, balance, credit_limit FROM accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AccountResponse
	for rows.Next() {
		var a AccountResponse
		var bal, cred int64
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.CurrencyCode, &bal, &cred); err != nil {
			return nil, err
		}
		a.Balance = float64(bal) / 100.0
		a.CreditLimit = float64(cred) / 100.0
		list = append(list, a)
	}
	return list, nil
}

type TransactionResponse struct {
	ID              string    `json:"id"`
	AccountID       string    `json:"account_id"`
	Time            time.Time `json:"time"`
	Description     string    `json:"description"`
	MCC             int       `json:"mcc"`
	Amount          float64   `json:"amount"`
	OperationAmount float64   `json:"operation_amount"`
	CurrencyCode    int       `json:"currency_code"`
	IsHold          bool      `json:"is_hold"`
}

func (s *Storage) GetTransactions(limit int) ([]TransactionResponse, error) {
	if limit <= 0 {
		limit = 10000
	}

	rows, err := s.db.Query(`
		SELECT id, account_id, time, description, mcc, amount, operation_amount, currency_code, is_hold 
		FROM transactions 
		ORDER BY time DESC 
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []TransactionResponse
	for rows.Next() {
		var t TransactionResponse
		var amt, opAmt int64
		if err := rows.Scan(&t.ID, &t.AccountID, &t.Time, &t.Description, &t.MCC, &amt, &opAmt, &t.CurrencyCode, &t.IsHold); err != nil {
			return nil, err
		}
		t.Amount = float64(amt) / 100.0
		t.OperationAmount = float64(opAmt) / 100.0
		list = append(list, t)
	}
	return list, nil
}

type SyncStatsResponse struct {
	TotalTransactions int    `json:"total_transactions"`
	OldestTxTime      string `json:"oldest_tx_time"`
	NewestTxTime      string `json:"newest_tx_time"`
}

func (s *Storage) GetSyncStats() (SyncStatsResponse, error) {
	var stats SyncStatsResponse
	var count int
	var minTime, maxTime sql.NullTime

	err := s.db.QueryRow(`
		SELECT COUNT(*), MIN(time), MAX(time) 
		FROM transactions
	`).Scan(&count, &minTime, &maxTime)

	if err != nil {
		return stats, err
	}

	stats.TotalTransactions = count
	if minTime.Valid {
		stats.OldestTxTime = minTime.Time.Format("2006-01-02 15:04:05")
	} else {
		stats.OldestTxTime = "—"
	}
	if maxTime.Valid {
		stats.NewestTxTime = maxTime.Time.Format("2006-01-02 15:04:05")
	} else {
		stats.NewestTxTime = "—"
	}

	return stats, nil
}
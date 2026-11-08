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

	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &Storage{db: db}
	if err := s.initTables(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			name TEXT,
			balance REAL,
			credit_limit REAL DEFAULT 0,
			type TEXT,
			currency_code INTEGER,
			iban TEXT,
			masked_pan TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			account_id TEXT,
			time INTEGER,
			description TEXT,
			comment TEXT,
			counter_edrpou TEXT,
			counter_iban TEXT,
			counter_name TEXT,
			mcc INTEGER,
			original_mcc INTEGER,
			amount REAL,
			operation_amount REAL,
			currency_code INTEGER,
			commission_rate REAL,
			cashback_amount REAL,
			is_hold BOOLEAN,
			resolved_user_id TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'DONOR',
			avatar_url TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS user_accounts (
			user_id TEXT NOT NULL,
			account_identifier TEXT NOT NULL PRIMARY KEY,
			note TEXT,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_aliases (
			user_id TEXT NOT NULL,
			alias TEXT NOT NULL PRIMARY KEY,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}

	return nil
}

type User struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	AvatarURL string   `json:"avatar_url"`
	IBANs     []string `json:"ibans"`
	Aliases   []string `json:"aliases"`
}

type Account struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Balance     float64 `json:"balance"`
	CreditLimit float64 `json:"credit_limit"`
	Type        string  `json:"type"`
	Currency    int     `json:"currency_code"`
	IBAN        string  `json:"iban"`
	MaskedPan   string  `json:"masked_pan"`
}

type Transaction struct {
	ID             string  `json:"id"`
	AccountID      string  `json:"account_id"`
	Time           string  `json:"time"`
	Description    string  `json:"description"`
	Comment        string  `json:"comment"`
	CounterIBAN    string  `json:"counter_iban"`
	CounterName    string  `json:"counter_name"`
	MCC            int     `json:"mcc"`
	Amount         float64 `json:"amount"`
	CurrencyCode   int     `json:"currency_code"`
	ResolvedUserID string  `json:"resolved_user_id,omitempty"`
}

func (s *Storage) SaveAccounts(accounts []mono.AccountDTO, ownerName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO accounts (id, name, balance, credit_limit, type, currency_code, iban, masked_pan)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			balance = excluded.balance,
			type = excluded.type,
			currency_code = excluded.currency_code
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, acc := range accounts {
		maskedPan := ""
		if len(acc.MaskedPan) > 0 {
			maskedPan = acc.MaskedPan[0]
		}

		accName := fmt.Sprintf("%s (%s)", ownerName, acc.Type)
		if ownerName == "" {
			accName = acc.Type
		}

		_, err := stmt.Exec(
			acc.ID,
			accName,
			float64(acc.Balance)/100.0,
			float64(acc.CreditLimit)/100.0,
			acc.Type,
			acc.CurrencyCode,
			acc.IBAN,
			maskedPan,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Storage) UpdateCreditLimit(accID string, creditLimit float64) error {
	_, err := s.db.Exec("UPDATE accounts SET credit_limit = ? WHERE id = ?", creditLimit, accID)
	return err
}

func (s *Storage) UpdateAccountManualDetails(accID, iban, cardNumber string, creditLimit float64) error {
	_, err := s.db.Exec(`
		UPDATE accounts 
		SET iban = CASE WHEN ? != '' THEN ? ELSE iban END,
		    masked_pan = CASE WHEN ? != '' THEN ? ELSE masked_pan END,
		    credit_limit = ?
		WHERE id = ?
	`, iban, iban, cardNumber, cardNumber, creditLimit, accID)
	return err
}

func (s *Storage) SaveTransactions(accID string, items []mono.StatementItemDTO) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO transactions (
			id, account_id, time, description, comment, counter_edrpou, counter_iban,
			counter_name, mcc, original_mcc, amount, operation_amount, currency_code,
			commission_rate, cashback_amount, is_hold
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			description = excluded.description,
			comment = excluded.comment,
			amount = excluded.amount
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		_, err := stmt.Exec(
			item.ID,
			accID,
			item.Time,
			item.Description,
			item.Comment,
			item.CounterEdrpou,
			item.CounterIban,
			item.CounterName,
			item.MCC,
			item.OriginalMCC,
			float64(item.Amount)/100.0,
			float64(item.OperationAmount)/100.0,
			item.CurrencyCode,
			float64(item.CommissionRate)/100.0,
			float64(item.CashbackAmount)/100.0,
			item.IsHold,
		)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.AutoResolveTransactions()
	return nil
}

func (s *Storage) AutoResolveTransactions() {
	s.db.Exec(`
		UPDATE transactions
		SET resolved_user_id = (
			SELECT ua.user_id 
			FROM user_accounts ua 
			WHERE LOWER(transactions.counter_iban) = LOWER(ua.account_identifier)
			LIMIT 1
		)
		WHERE counter_iban IS NOT NULL AND counter_iban != '' AND resolved_user_id IS NULL;
	`)

	s.db.Exec(`
		UPDATE transactions
		SET resolved_user_id = (
			SELECT ual.user_id 
			FROM user_aliases ual 
			WHERE LOWER(transactions.counter_name) LIKE '%' || LOWER(ual.alias) || '%'
			   OR LOWER(transactions.description) LIKE '%' || LOWER(ual.alias) || '%'
			LIMIT 1
		)
		WHERE resolved_user_id IS NULL;
	`)
}

func (s *Storage) GetUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, name, role, COALESCE(avatar_url, '') FROM users ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Role, &u.AvatarURL); err != nil {
			return nil, err
		}
		u.IBANs = s.getUserIBANs(u.ID)
		u.Aliases = s.getUserAliases(u.ID)
		users = append(users, u)
	}
	return users, nil
}

func (s *Storage) getUserIBANs(userID string) []string {
	rows, err := s.db.Query("SELECT account_identifier FROM user_accounts WHERE user_id = ?", userID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var list []string
	for rows.Next() {
		var val string
		rows.Scan(&val)
		list = append(list, val)
	}
	return list
}

func (s *Storage) getUserAliases(userID string) []string {
	rows, err := s.db.Query("SELECT alias FROM user_aliases WHERE user_id = ?", userID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var list []string
	for rows.Next() {
		var val string
		rows.Scan(&val)
		list = append(list, val)
	}
	return list
}

func (s *Storage) CreateUser(u User) error {
	_, err := s.db.Exec("INSERT INTO users (id, name, role, avatar_url) VALUES (?, ?, ?, ?)", u.ID, u.Name, u.Role, u.AvatarURL)
	return err
}

func (s *Storage) UpdateUser(u User) error {
	_, err := s.db.Exec("UPDATE users SET name = ?, role = ?, avatar_url = ? WHERE id = ?", u.Name, u.Role, u.AvatarURL, u.ID)
	return err
}

func (s *Storage) DeleteUser(id string) error {
	_, err := s.db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

func (s *Storage) LinkAccountToUser(userID, identifier, note string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_accounts (user_id, account_identifier, note) VALUES (?, ?, ?)
		ON CONFLICT(account_identifier) DO UPDATE SET user_id = excluded.user_id, note = excluded.note
	`, userID, identifier, note)
	if err == nil {
		s.AutoResolveTransactions()
	}
	return err
}

func (s *Storage) UnlinkAccountFromUser(identifier string) error {
	_, err := s.db.Exec("DELETE FROM user_accounts WHERE account_identifier = ?", identifier)
	if err == nil {
		s.db.Exec("UPDATE transactions SET resolved_user_id = NULL WHERE counter_iban = ?", identifier)
	}
	return err
}

func (s *Storage) AddUserAlias(userID, alias string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_aliases (user_id, alias) VALUES (?, ?)
		ON CONFLICT(alias) DO UPDATE SET user_id = excluded.user_id
	`, userID, alias)
	if err == nil {
		s.AutoResolveTransactions()
	}
	return err
}

func (s *Storage) RemoveUserAlias(alias string) error {
	_, err := s.db.Exec("DELETE FROM user_aliases WHERE alias = ?", alias)
	if err == nil {
		s.db.Exec("UPDATE transactions SET resolved_user_id = NULL WHERE counter_name LIKE '%' || ? || '%' OR description LIKE '%' || ? || '%'", alias, alias)
	}
	return err
}

func (s *Storage) GetAccounts() ([]Account, error) {
	rows, err := s.db.Query("SELECT id, name, balance, credit_limit, type, currency_code, COALESCE(iban, ''), COALESCE(masked_pan, '') FROM accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Balance, &a.CreditLimit, &a.Type, &a.Currency, &a.IBAN, &a.MaskedPan); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (s *Storage) GetTransactions(limit int) ([]Transaction, error) {
	rows, err := s.db.Query(`
		SELECT id, account_id, time, COALESCE(description, ''), COALESCE(comment, ''),
		       COALESCE(counter_iban, ''), COALESCE(counter_name, ''), mcc, amount, currency_code,
		       COALESCE(resolved_user_id, '')
		FROM transactions
		ORDER BY time DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Transaction
	for rows.Next() {
		var t Transaction
		var timeUnix int64
		if err := rows.Scan(&t.ID, &t.AccountID, &timeUnix, &t.Description, &t.Comment, &t.CounterIBAN, &t.CounterName, &t.MCC, &t.Amount, &t.CurrencyCode, &t.ResolvedUserID); err != nil {
			return nil, err
		}
		t.Time = time.Unix(timeUnix, 0).Format(time.RFC3339)
		list = append(list, t)
	}
	return list, nil
}

func (s *Storage) GetSyncStats() (map[string]interface{}, error) {
	var total int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&total)

	var oldestUnix, newestUnix sql.NullInt64
	_ = s.db.QueryRow("SELECT MIN(time), MAX(time) FROM transactions").Scan(&oldestUnix, &newestUnix)

	oldestStr := "—"
	newestStr := "—"
	if oldestUnix.Valid {
		oldestStr = time.Unix(oldestUnix.Int64, 0).Format("02.01.2006")
	}
	if newestUnix.Valid {
		newestStr = time.Unix(newestUnix.Int64, 0).Format("02.01.2006")
	}

	return map[string]interface{}{
		"total_transactions": total,
		"oldest_tx_time":     oldestStr,
		"newest_tx_time":     newestStr,
	}, nil
}
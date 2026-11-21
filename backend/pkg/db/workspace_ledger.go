package db

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Учёт дела (Core DB): какие счета входят в дело, бейджи на операциях этих счетов и понедельные записи номинального дохода.
// Операции и счета не копируются: дело хранит только ссылки (id счёта, id операции) и свои пометки.
// Набор бейджей и их смысл задаёт страница дела на фронте; здесь проверяется только вид ключа.

var workspaceBadge = regexp.MustCompile(`^[a-z][a-z0-9-]{0,15}$`)

const maxWeeklyNominal = 10_000_000

var workspaceParamKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

const maxWorkspaceParam = 1_000_000_000

// WorkspaceMark — бейдж на одной операции. Операция принадлежит ровно одному делу: повторная пометка из другого дела переносит её.
type WorkspaceMark struct {
	TxID        string `json:"tx_id"`
	WorkspaceID string `json:"workspace_id"`
	Badge       string `json:"badge"`
}

// WorkspaceWeek — запись «за неделю (с понедельника week) Glovo записал столько номинального дохода».
type WorkspaceWeek struct {
	Week    string  `json:"week"`    // понедельник недели, YYYY-MM-DD
	Nominal float64 `json:"nominal"` // сумма до налога и вычетов
}

// SetWorkspaceAccounts заменяет список счетов дела целиком (повторы и пустые id отбрасываются).
func (s *Storage) SetWorkspaceAccounts(wsID string, ids []string) error {
	if wsID == "" {
		return fmt.Errorf("%w: workspace id is required", ErrWorkspace)
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM workspace_accounts WHERE workspace_id = ?`, wsID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if _, err := tx.Exec(`INSERT INTO workspace_accounts(workspace_id, account_id) VALUES(?,?)`, wsID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetWorkspaceAccounts — id счетов дела (по порядку id; пусто — []).
func (s *Storage) GetWorkspaceAccounts(wsID string) ([]string, error) {
	rows, err := s.coreDB.Query(`SELECT account_id FROM workspace_accounts WHERE workspace_id = ? ORDER BY account_id`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetWorkspaceMark ставит бейдж на операцию; пустой бейдж снимает пометку. Операция одна на всё приложение: из другого дела она переезжает.
func (s *Storage) SetWorkspaceMark(wsID, txID, badge string) error {
	if wsID == "" || txID == "" {
		return fmt.Errorf("%w: workspace id and tx id are required", ErrWorkspace)
	}
	if badge != "" && !workspaceBadge.MatchString(badge) {
		return fmt.Errorf("%w: badge must be a lowercase latin key up to 16 chars", ErrWorkspace)
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	if badge == "" {
		_, err := s.coreDB.Exec(`DELETE FROM workspace_marks WHERE tx_id = ? AND workspace_id = ?`, txID, wsID)
		return err
	}
	_, err := s.coreDB.Exec(`INSERT INTO workspace_marks(tx_id, workspace_id, badge) VALUES(?,?,?)
		ON CONFLICT(tx_id) DO UPDATE SET workspace_id = excluded.workspace_id, badge = excluded.badge`, txID, wsID, badge)
	return err
}

// GetWorkspaceMarks — все пометки дела.
func (s *Storage) GetWorkspaceMarks(wsID string) ([]WorkspaceMark, error) {
	return s.queryMarks(`WHERE workspace_id = ?`, wsID)
}

// GetOtherWorkspaceMarks — пометки всех остальных дел: фронт серит в истории операции, уже разобранные соседними делами.
func (s *Storage) GetOtherWorkspaceMarks(wsID string) ([]WorkspaceMark, error) {
	return s.queryMarks(`WHERE workspace_id <> ?`, wsID)
}

func (s *Storage) queryMarks(where string, arg string) ([]WorkspaceMark, error) {
	rows, err := s.coreDB.Query(`SELECT tx_id, workspace_id, badge FROM workspace_marks `+where+` ORDER BY tx_id`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkspaceMark{}
	for rows.Next() {
		var m WorkspaceMark
		if err := rows.Scan(&m.TxID, &m.WorkspaceID, &m.Badge); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetWorkspaceWeek записывает номинальный доход за неделю (повтор той же недели заменяет значение).
func (s *Storage) SetWorkspaceWeek(wsID string, w WorkspaceWeek) error {
	if wsID == "" {
		return fmt.Errorf("%w: workspace id is required", ErrWorkspace)
	}
	d, err := time.Parse("2006-01-02", w.Week)
	if err != nil || d.Weekday() != time.Monday {
		return fmt.Errorf("%w: week must be a Monday as YYYY-MM-DD", ErrWorkspace)
	}
	if math.IsNaN(w.Nominal) || math.IsInf(w.Nominal, 0) || w.Nominal < 0 || w.Nominal > maxWeeklyNominal {
		return fmt.Errorf("%w: nominal must be between 0 and %d", ErrWorkspace, maxWeeklyNominal)
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err = s.coreDB.Exec(`INSERT INTO workspace_weeks(workspace_id, week, nominal) VALUES(?,?,?)
		ON CONFLICT(workspace_id, week) DO UPDATE SET nominal = excluded.nominal`, wsID, w.Week, w.Nominal)
	return err
}

// DeleteWorkspaceWeek удаляет запись недели (нет такой — не ошибка).
func (s *Storage) DeleteWorkspaceWeek(wsID, week string) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`DELETE FROM workspace_weeks WHERE workspace_id = ? AND week = ?`, wsID, week)
	return err
}

// GetWorkspaceWeeks — записи недель по возрастанию даты.
func (s *Storage) GetWorkspaceWeeks(wsID string) ([]WorkspaceWeek, error) {
	rows, err := s.coreDB.Query(`SELECT week, nominal FROM workspace_weeks WHERE workspace_id = ? ORDER BY week`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkspaceWeek{}
	for rows.Next() {
		var w WorkspaceWeek
		if err := rows.Scan(&w.Week, &w.Nominal); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Week < out[j].Week })
	return out, rows.Err()
}

// WorkspaceParam — числовой параметр дела (например, ставки налога ФОП). Смысл ключей задаёт страница дела на фронте.
type WorkspaceParam struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
}

// SetWorkspaceParam записывает параметр дела (повтор ключа заменяет значение).
func (s *Storage) SetWorkspaceParam(wsID string, p WorkspaceParam) error {
	if wsID == "" {
		return fmt.Errorf("%w: workspace id is required", ErrWorkspace)
	}
	if !workspaceParamKey.MatchString(p.Key) {
		return fmt.Errorf("%w: bad param key", ErrWorkspace)
	}
	if math.IsNaN(p.Value) || math.IsInf(p.Value, 0) || p.Value < 0 || p.Value > maxWorkspaceParam {
		return fmt.Errorf("%w: param must be between 0 and %d", ErrWorkspace, maxWorkspaceParam)
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`INSERT INTO workspace_params(workspace_id, key, value) VALUES(?,?,?)
		ON CONFLICT(workspace_id, key) DO UPDATE SET value = excluded.value`, wsID, p.Key, p.Value)
	return err
}

// DeleteWorkspaceParam убирает параметр (нет такого — не ошибка): страница вернётся к значению по умолчанию.
func (s *Storage) DeleteWorkspaceParam(wsID, key string) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`DELETE FROM workspace_params WHERE workspace_id = ? AND key = ?`, wsID, key)
	return err
}

// GetWorkspaceParams — параметры дела по возрастанию ключа.
func (s *Storage) GetWorkspaceParams(wsID string) ([]WorkspaceParam, error) {
	rows, err := s.coreDB.Query(`SELECT key, value FROM workspace_params WHERE workspace_id = ? ORDER BY key`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkspaceParam{}
	for rows.Next() {
		var p WorkspaceParam
		if err := rows.Scan(&p.Key, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// WorkspaceEntry — ручная запись дела: деньги, которых нет в выписках (наличные расходы, компенсация и т. п.).
// Amount — положительная величина; приход это или уход, решает бейдж на странице дела.
// TxID — операция, к которой запись прикреплена (на странице она рисуется под ней); пусто — запись без операции.
type WorkspaceEntry struct {
	ID     string  `json:"id"`
	Day    string  `json:"day"` // YYYY-MM-DD
	Badge  string  `json:"badge"`
	Amount float64 `json:"amount"`
	Note   string  `json:"note"`
	TxID   string  `json:"tx_id"`
	// Src — id документа другого дела, из которого запись возникла (сжигание партии ХимЛаба); пусто — ручная запись.
	// Запись с источником правке и удалению напрямую не поддаётся, она уходит вместе с документом.
	Src string `json:"src"`
}

const (
	maxEntryNote = 200
	maxEntryTxID = 128
	maxEntrySrc  = 64
)

func (e WorkspaceEntry) clean() (WorkspaceEntry, error) {
	e.Note = strings.TrimSpace(e.Note)
	e.TxID = strings.TrimSpace(e.TxID)
	e.Src = strings.TrimSpace(e.Src)
	if len(e.Src) > maxEntrySrc {
		return e, fmt.Errorf("%w: src is longer than %d characters", ErrWorkspace, maxEntrySrc)
	}
	if len(e.TxID) > maxEntryTxID {
		return e, fmt.Errorf("%w: tx id is longer than %d characters", ErrWorkspace, maxEntryTxID)
	}
	if utf8.RuneCountInString(e.Note) > maxEntryNote {
		return e, fmt.Errorf("%w: note is longer than %d characters", ErrWorkspace, maxEntryNote)
	}
	if _, err := time.Parse("2006-01-02", e.Day); err != nil {
		return e, fmt.Errorf("%w: day must be YYYY-MM-DD", ErrWorkspace)
	}
	if !workspaceBadge.MatchString(e.Badge) {
		return e, fmt.Errorf("%w: bad badge key", ErrWorkspace)
	}
	if math.IsNaN(e.Amount) || math.IsInf(e.Amount, 0) || e.Amount <= 0 || e.Amount > maxWeeklyNominal {
		return e, fmt.Errorf("%w: amount must be above 0 and at most %d", ErrWorkspace, maxWeeklyNominal)
	}
	e.Amount = math.Round(e.Amount*100) / 100
	return e, nil
}

// CreateWorkspaceEntry добавляет ручную запись; id ставит сервер.
func (s *Storage) CreateWorkspaceEntry(wsID string, e WorkspaceEntry) (WorkspaceEntry, error) {
	if wsID == "" {
		return e, fmt.Errorf("%w: workspace id is required", ErrWorkspace)
	}
	e, err := e.clean()
	if err != nil {
		return e, err
	}
	e.ID = fmt.Sprintf("we_%d", time.Now().UnixNano())
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err = s.coreDB.Exec(`INSERT INTO workspace_entries(id, workspace_id, day, badge, amount, note, tx_id, src) VALUES(?,?,?,?,?,?,?,?)`, e.ID, wsID, e.Day, e.Badge, e.Amount, e.Note, e.TxID, e.Src)
	return e, err
}

// UpdateWorkspaceEntry правит ручную запись своего дела; false — такой записи нет (запись с источником тоже не правится).
func (s *Storage) UpdateWorkspaceEntry(wsID string, e WorkspaceEntry) (bool, error) {
	e, err := e.clean()
	if err != nil {
		return false, err
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	res, err := s.coreDB.Exec(`UPDATE workspace_entries SET day=?, badge=?, amount=?, note=?, tx_id=? WHERE id=? AND workspace_id=? AND src=''`, e.Day, e.Badge, e.Amount, e.Note, e.TxID, e.ID, wsID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteWorkspaceEntry удаляет ручную запись своего дела (нет такой — не ошибка; запись с источником удаляется вместе с документом).
func (s *Storage) DeleteWorkspaceEntry(wsID, id string) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err := s.coreDB.Exec(`DELETE FROM workspace_entries WHERE id = ? AND workspace_id = ? AND src = ''`, id, wsID)
	return err
}

// GetWorkspaceEntries — записи дела по дате (внутри даты по порядку добавления).
func (s *Storage) GetWorkspaceEntries(wsID string) ([]WorkspaceEntry, error) {
	rows, err := s.coreDB.Query(`SELECT id, day, badge, amount, note, tx_id, src FROM workspace_entries WHERE workspace_id = ? ORDER BY day, id`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkspaceEntry{}
	for rows.Next() {
		var e WorkspaceEntry
		if err := rows.Scan(&e.ID, &e.Day, &e.Badge, &e.Amount, &e.Note, &e.TxID, &e.Src); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

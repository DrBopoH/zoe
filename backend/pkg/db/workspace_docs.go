package db

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// Документы дела (Core DB): универсальное хранилище записей страницы дела. Вид (kind) и содержимое (JSON-объект) задаёт страница на фронте,
// например страница «ХимЛаб» хранит карточки товаров, партии и движения партий. Бэкенд проверяет только вид, дату и размер,
// поэтому новая сущность страницы не требует миграции. Операции и счета, на которые документ ссылается, лежат внутри его JSON.

var workspaceDocKind = regexp.MustCompile(`^[a-z][a-z0-9_]{0,23}$`)

const maxDocBytes = 16 * 1024

// WorkspaceDoc — один документ дела. Day — необязательная дата YYYY-MM-DD (для порядка в списках), Data — JSON-объект.
type WorkspaceDoc struct {
	ID   string          `json:"id"`
	Kind string          `json:"kind"`
	Day  string          `json:"day"`
	Data json.RawMessage `json:"data"`
}

func (d WorkspaceDoc) clean() (WorkspaceDoc, error) {
	if !workspaceDocKind.MatchString(d.Kind) {
		return d, fmt.Errorf("%w: bad doc kind", ErrWorkspace)
	}
	if d.Day != "" {
		if _, err := time.Parse("2006-01-02", d.Day); err != nil {
			return d, fmt.Errorf("%w: day must be YYYY-MM-DD or empty", ErrWorkspace)
		}
	}
	if len(d.Data) > maxDocBytes {
		return d, fmt.Errorf("%w: doc is larger than %d bytes", ErrWorkspace, maxDocBytes)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(d.Data, &obj); err != nil || obj == nil {
		return d, fmt.Errorf("%w: doc data must be a JSON object", ErrWorkspace)
	}
	return d, nil
}

// CreateWorkspaceDoc добавляет документ; id ставит сервер.
func (s *Storage) CreateWorkspaceDoc(wsID string, d WorkspaceDoc) (WorkspaceDoc, error) {
	if wsID == "" {
		return d, fmt.Errorf("%w: workspace id is required", ErrWorkspace)
	}
	d, err := d.clean()
	if err != nil {
		return d, err
	}
	d.ID = fmt.Sprintf("wd_%d", time.Now().UnixNano())
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err = s.coreDB.Exec(`INSERT INTO workspace_docs(id, workspace_id, kind, day, data) VALUES(?,?,?,?,?)`, d.ID, wsID, d.Kind, d.Day, string(d.Data))
	return d, err
}

// UpdateWorkspaceDoc заменяет документ своего дела (вид не меняется); false — такого документа нет.
func (s *Storage) UpdateWorkspaceDoc(wsID string, d WorkspaceDoc) (bool, error) {
	d, err := d.clean()
	if err != nil {
		return false, err
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	res, err := s.coreDB.Exec(`UPDATE workspace_docs SET day=?, data=? WHERE id=? AND workspace_id=? AND kind=?`, d.Day, string(d.Data), d.ID, wsID, d.Kind)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteWorkspaceDoc удаляет документ своего дела (нет такого — не ошибка) вместе с записями других дел, которые он породил.
func (s *Storage) DeleteWorkspaceDoc(wsID, id string) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// записи других дел, возникшие из этого документа (сжигание партии), уходят вместе с ним
	if _, err := tx.Exec(`DELETE FROM workspace_entries WHERE src = ? AND ? IN (SELECT workspace_id FROM workspace_docs WHERE id = ?)`, id, wsID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM workspace_docs WHERE id = ? AND workspace_id = ?`, id, wsID); err != nil {
		return err
	}
	return tx.Commit()
}

// GetWorkspaceDocs — документы дела (kind пустой — все виды) по дате, внутри даты по порядку добавления.
func (s *Storage) GetWorkspaceDocs(wsID, kind string) ([]WorkspaceDoc, error) {
	q := `SELECT id, kind, day, data FROM workspace_docs WHERE workspace_id = ?`
	args := []any{wsID}
	if kind != "" {
		q += ` AND kind = ?`
		args = append(args, kind)
	}
	rows, err := s.coreDB.Query(q+` ORDER BY day, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkspaceDoc{}
	for rows.Next() {
		var d WorkspaceDoc
		var data string
		if err := rows.Scan(&d.ID, &d.Kind, &d.Day, &data); err != nil {
			return nil, err
		}
		d.Data = json.RawMessage(data)
		out = append(out, d)
	}
	return out, rows.Err()
}

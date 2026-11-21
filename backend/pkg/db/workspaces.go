package db

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Дела (Core DB): рабочие пространства пользователя. Дело ссылается на остальные данные (счета, лица, операции), а своё хранит локально.
// Пока это имя, обложка и ключ страницы; пометки операций и правила привязки придут следующими шагами.

type Workspace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"` // обложка: #rrggbb или пусто
	Image     string `json:"image"` // обложка: ссылка http(s) или пусто; поверх цвета
	Page      string `json:"page"`  // ключ страницы дела на фронте (реестр страниц); пусто — страницы пока нет
	CreatedAt int64  `json:"created_at"`
}

const (
	maxWorkspaceName     = 60
	maxWorkspaceImageURL = 2000 // символов в ссылке на обложку
)

var (
	workspaceColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	// ключ страницы: сами страницы живут на фронте, здесь проверяется только вид ключа
	workspacePage = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	// ссылка на картинку: без пробелов, кавычек и скобок — её кладут в CSS url("…")
	workspaceImageURL = regexp.MustCompile(`^https?://[^\s"'()\\]+$`)
)

// ErrWorkspace — неверные поля дела (ответ 400, а не сбой сервера).
var ErrWorkspace = errors.New("workspace")

// Clean приводит поля к виду для записи и проверяет их: имя обязательно и не длиннее 60 символов, цвет — #rrggbb, картинка — небольшой data-URL.
func (w Workspace) Clean() (Workspace, error) {
	w.Name = strings.Join(strings.Fields(w.Name), " ")
	w.Color = strings.TrimSpace(w.Color)
	w.Image = strings.TrimSpace(w.Image)
	w.Page = strings.TrimSpace(w.Page)
	switch {
	case w.Name == "":
		return w, fmt.Errorf("%w: name is required", ErrWorkspace)
	case utf8.RuneCountInString(w.Name) > maxWorkspaceName:
		return w, fmt.Errorf("%w: name is longer than %d", ErrWorkspace, maxWorkspaceName)
	case w.Color != "" && !workspaceColor.MatchString(w.Color):
		return w, fmt.Errorf("%w: color must be #rrggbb", ErrWorkspace)
	case w.Page != "" && !workspacePage.MatchString(w.Page):
		return w, fmt.Errorf("%w: page must be a lowercase latin key up to 32 chars", ErrWorkspace)
	case w.Image != "" && !validWorkspaceImage(w.Image):
		return w, fmt.Errorf("%w: image must be an http(s) link up to %d chars", ErrWorkspace, maxWorkspaceImageURL)
	}
	return w, nil
}

// CreateWorkspace записывает новое дело; id и время создания выставляются здесь, если не заданы.
func (s *Storage) CreateWorkspace(w Workspace) (Workspace, error) {
	w, err := w.Clean()
	if err != nil {
		return w, err
	}
	now := time.Now()
	if w.ID == "" {
		w.ID = fmt.Sprintf("ws_%d", now.UnixNano())
	}
	if w.CreatedAt == 0 {
		w.CreatedAt = now.UnixMilli()
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	_, err = s.coreDB.Exec(`INSERT INTO workspaces(id, name, color, image, page, created_at) VALUES(?,?,?,?,?,?)`, w.ID, w.Name, w.Color, w.Image, w.Page, w.CreatedAt)
	return w, err
}

// UpdateWorkspace правит имя, обложку и страницу; возвращает false, если такого дела нет.
func (s *Storage) UpdateWorkspace(w Workspace) (bool, error) {
	w, err := w.Clean()
	if err != nil {
		return false, err
	}
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	res, err := s.coreDB.Exec(`UPDATE workspaces SET name=?, color=?, image=?, page=? WHERE id=?`, w.Name, w.Color, w.Image, w.Page, w.ID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// DeleteWorkspace удаляет дело по id (нет такого — не ошибка).
func (s *Storage) DeleteWorkspace(id string) error {
	defer s.bump()
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	tx, err := s.coreDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// вместе с делом уходит и его учёт: счета, пометки операций, недельные записи
	for _, q := range []string{
		`DELETE FROM workspace_accounts WHERE workspace_id = ?`,
		`DELETE FROM workspace_marks WHERE workspace_id = ?`,
		`DELETE FROM workspace_weeks WHERE workspace_id = ?`,
		`DELETE FROM workspace_params WHERE workspace_id = ?`,
		`DELETE FROM workspace_entries WHERE workspace_id = ?`,
		`DELETE FROM workspace_docs WHERE workspace_id = ?`,
		`DELETE FROM workspaces WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetWorkspaces — все дела по порядку создания (старые первыми).
func (s *Storage) GetWorkspaces() ([]Workspace, error) {
	rows, err := s.coreDB.Query(`SELECT id, name, color, image, page, created_at FROM workspaces ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Workspace{}
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Color, &w.Image, &w.Page, &w.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

func validWorkspaceImage(img string) bool {
	return len(img) <= maxWorkspaceImageURL && workspaceImageURL.MatchString(img)
}

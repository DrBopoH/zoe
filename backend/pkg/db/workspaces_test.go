package db

import (
	"errors"
	"strings"
	"testing"
)

func TestWorkspacesCreateListUpdateDelete(t *testing.T) {
	s, _, _ := newS(t)
	defer s.Close()
	if list, err := s.GetWorkspaces(); err != nil || len(list) != 0 || list == nil {
		t.Fatalf("пустой список должен быть [], а не nil: %v %v", list, err)
	}
	a, err := s.CreateWorkspace(Workspace{Name: "  Glovo   курьер ", Color: "#10B981", Page: "glovo"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Glovo курьер" || a.ID == "" || a.CreatedAt == 0 {
		t.Fatalf("имя чистится от лишних пробелов, id и время ставит сервер: %+v", a)
	}
	b, err := s.CreateWorkspace(Workspace{Name: "Семейный бюджет", CreatedAt: a.CreatedAt + 5})
	if err != nil {
		t.Fatal(err)
	}
	list, _ := s.GetWorkspaces()
	if list[0].Page != "glovo" || list[1].Page != "" {
		t.Fatalf("ключ страницы хранится и читается: %+v", list)
	}
	if len(list) != 2 || list[0].ID != a.ID || list[1].ID != b.ID {
		t.Fatalf("порядок — по созданию: %+v", list)
	}

	ver := s.ver.Load()
	ok, err := s.UpdateWorkspace(Workspace{ID: a.ID, Name: "Glovo", Color: "", Page: ""})
	if err != nil || !ok {
		t.Fatalf("правка: %v %v", ok, err)
	}
	if s.ver.Load() == ver {
		t.Fatal("запись должна менять версию данных, иначе фронт не перечитает")
	}
	if ok, _ := s.UpdateWorkspace(Workspace{ID: "ws_none", Name: "x"}); ok {
		t.Fatal("правка несуществующего дела не должна проходить")
	}
	list, _ = s.GetWorkspaces()
	if list[0].Name != "Glovo" || list[0].Color != "" || list[0].Page != "" {
		t.Fatalf("после правки: %+v", list[0])
	}

	if err := s.DeleteWorkspace(a.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspace("ws_none"); err != nil {
		t.Fatalf("удалить несуществующее — не ошибка: %v", err)
	}
	if list, _ = s.GetWorkspaces(); len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("после удаления: %+v", list)
	}
}

func TestWorkspaceValidation(t *testing.T) {
	img := "https://example.com/cover.png"
	bad := map[string]Workspace{
		"пустое имя":          {Name: "   "},
		"длинное имя":         {Name: strings.Repeat("я", maxWorkspaceName+1)},
		"цвет не hex":         {Name: "x", Color: "red"},
		"цвет короткий":       {Name: "x", Color: "#fff"},
		"картинка data":       {Name: "x", Image: "data:image/png;base64,iVBORw0KGgo="},
		"картинка js":         {Name: "x", Image: "javascript:alert(1)"},
		"страница заглавными": {Name: "x", Page: "Glovo"},
		"страница с пробелом": {Name: "x", Page: "glovo page"},
		"страница длинная":    {Name: "x", Page: strings.Repeat("a", 33)},
		"картинка ftp":        {Name: "x", Image: "ftp://example.com/a.png"},
		"url с кавычкой":      {Name: "x", Image: "https://e.com/a\".png"},
		"url со скобкой":      {Name: "x", Image: "https://e.com/a).png"},
		"url длинный":         {Name: "x", Image: "https://e.com/" + strings.Repeat("a", maxWorkspaceImageURL)},
	}
	for name, w := range bad {
		if _, err := w.Clean(); !errors.Is(err, ErrWorkspace) {
			t.Errorf("%s: ждали ErrWorkspace, получили %v", name, err)
		}
	}
	good := []Workspace{{Name: "я"}, {Name: strings.Repeat("я", maxWorkspaceName)}, {Name: "x", Color: "#00aaFF"}, {Name: "x", Image: img}, {Name: "x", Color: "#000000", Image: img}, {Name: "x", Image: "https://example.com/a.png?x=1&y=2"}}
	for _, w := range good {
		if _, err := w.Clean(); err != nil {
			t.Errorf("%+v: %v", w, err)
		}
	}
}

func TestWorkspacePageColumnMigration(t *testing.T) {
	s, _, _ := newS(t)
	defer s.Close()
	// база, созданная до появления ключа страницы: таблица без столбца page
	for _, q := range []string{
		`DROP TABLE workspaces`,
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL, color TEXT NOT NULL DEFAULT '', image TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL)`,
		`INSERT INTO workspaces(id, name, created_at) VALUES('ws_old', 'Старое', 1)`,
	} {
		if _, err := s.coreDB.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ { // повторный запуск ничего не ломает
		if err := s.ensureWorkspacePageColumn(); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.GetWorkspaces()
	if err != nil || len(list) != 1 || list[0].Name != "Старое" || list[0].Page != "" {
		t.Fatalf("старое дело читается с пустой страницей: %+v %v", list, err)
	}
}

func TestWorkspaceEntryTxColumnMigration(t *testing.T) {
	s, _, _ := newS(t)
	defer s.Close()
	// база с ручными записями, созданными до привязки к операции: таблица без столбца tx_id
	for _, q := range []string{
		`DROP TABLE workspace_entries`,
		`CREATE TABLE workspace_entries (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, day TEXT NOT NULL, badge TEXT NOT NULL, amount REAL NOT NULL, note TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO workspace_entries(id, workspace_id, day, badge, amount, note) VALUES('we_old', 'ws', '2026-11-10', 'loss', 900, 'ремонт')`,
	} {
		if _, err := s.coreDB.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ { // повторный запуск ничего не ломает
		if err := s.ensureWorkspaceEntryTxColumn(); err != nil {
			t.Fatal(err)
		}
	}
	es, err := s.GetWorkspaceEntries("ws")
	if err != nil || len(es) != 1 || es[0].Note != "ремонт" || es[0].TxID != "" {
		t.Fatalf("старая запись читается без операции: %+v %v", es, err)
	}
}

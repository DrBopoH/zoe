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
	a, err := s.CreateWorkspace(Workspace{Name: "  Glovo   курьер ", Color: "#10B981"})
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
	if len(list) != 2 || list[0].ID != a.ID || list[1].ID != b.ID {
		t.Fatalf("порядок — по созданию: %+v", list)
	}

	ver := s.ver.Load()
	ok, err := s.UpdateWorkspace(Workspace{ID: a.ID, Name: "Glovo", Color: ""})
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
	if list[0].Name != "Glovo" || list[0].Color != "" {
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
	img := "data:image/png;base64,iVBORw0KGgo="
	bad := map[string]Workspace{
		"пустое имя":       {Name: "   "},
		"длинное имя":      {Name: strings.Repeat("я", maxWorkspaceName+1)},
		"цвет не hex":      {Name: "x", Color: "red"},
		"цвет короткий":    {Name: "x", Color: "#fff"},
		"картинка не img":  {Name: "x", Image: "data:text/html;base64,PGI+"},
		"картинка url":     {Name: "x", Image: "https://example.com/a.png"},
		"картинка svg":     {Name: "x", Image: "data:image/svg+xml;base64,PHN2Zz4="},
		"картинка большая": {Name: "x", Image: "data:image/png;base64," + strings.Repeat("A", maxWorkspaceImage)},
	}
	for name, w := range bad {
		if _, err := w.Clean(); !errors.Is(err, ErrWorkspace) {
			t.Errorf("%s: ждали ErrWorkspace, получили %v", name, err)
		}
	}
	good := []Workspace{{Name: "я"}, {Name: strings.Repeat("я", maxWorkspaceName)}, {Name: "x", Color: "#00aaFF"}, {Name: "x", Image: img}, {Name: "x", Color: "#000000", Image: img}}
	for _, w := range good {
		if _, err := w.Clean(); err != nil {
			t.Errorf("%+v: %v", w, err)
		}
	}
}

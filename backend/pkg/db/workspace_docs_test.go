package db

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestWorkspaceDocs(t *testing.T) {
	s, _, _ := newS(t)
	defer s.Close()
	a, _ := s.CreateWorkspace(Workspace{Name: "ХимЛаб"})
	b, _ := s.CreateWorkspace(Workspace{Name: "Другое"})

	card := json.RawMessage(`{"tx_id":"t1","substance":"мальтодекстрин","net_g":1000,"price":96,"qty":5}`)
	d, err := s.CreateWorkspaceDoc(a.ID, WorkspaceDoc{Kind: "card", Day: "2026-11-10", Data: card})
	if err != nil || d.ID == "" {
		t.Fatalf("документ: %+v %v", d, err)
	}
	if _, err := s.CreateWorkspaceDoc(a.ID, WorkspaceDoc{Kind: "batch", Day: "2026-11-12", Data: json.RawMessage(`{"code":"261112-1"}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateWorkspaceDoc(b.ID, WorkspaceDoc{Kind: "card", Data: card}); err != nil {
		t.Fatal(err)
	}

	// плохие документы: вид, дата, не объект, слишком большой, без дела
	for _, bad := range []WorkspaceDoc{
		{Kind: "Card", Data: card},
		{Kind: "", Data: card},
		{Kind: "card", Day: "10.11.2026", Data: card},
		{Kind: "card", Data: json.RawMessage(`[1,2]`)},
		{Kind: "card", Data: json.RawMessage(`"текст"`)},
		{Kind: "card", Data: json.RawMessage(`null`)},
		{Kind: "card", Data: json.RawMessage(`{"a":"` + strings.Repeat("я", maxDocBytes) + `"}`)},
	} {
		if _, err := s.CreateWorkspaceDoc(a.ID, bad); !errors.Is(err, ErrWorkspace) {
			t.Fatalf("должна быть ошибка для %.40s: %v", string(bad.Data), err)
		}
	}
	if _, err := s.CreateWorkspaceDoc("", WorkspaceDoc{Kind: "card", Data: card}); !errors.Is(err, ErrWorkspace) {
		t.Fatalf("без дела: %v", err)
	}

	// чтение: по виду и все, по дате, чужое дело не видно
	if l, _ := s.GetWorkspaceDocs(a.ID, "card"); len(l) != 1 || !strings.Contains(string(l[0].Data), "мальтодекстрин") {
		t.Fatalf("карточки: %+v", l)
	}
	if l, _ := s.GetWorkspaceDocs(a.ID, ""); len(l) != 2 || l[0].Kind != "card" || l[1].Kind != "batch" {
		t.Fatalf("все документы по дате: %+v", l)
	}
	if l, err := s.GetWorkspaceDocs("none", ""); err != nil || l == nil || len(l) != 0 {
		t.Fatalf("пусто — [], не nil: %v %v", l, err)
	}

	// правка: свой документ, свой вид; чужое дело и чужой вид не правят
	upd := WorkspaceDoc{ID: d.ID, Kind: "card", Day: "2026-11-11", Data: json.RawMessage(`{"tx_id":"t1","substance":"мальтодекстрин","net_g":1000,"price":99,"qty":5}`)}
	if ok, err := s.UpdateWorkspaceDoc(b.ID, upd); err != nil || ok {
		t.Fatalf("чужое дело: %v %v", ok, err)
	}
	other := upd
	other.Kind = "batch"
	if ok, err := s.UpdateWorkspaceDoc(a.ID, other); err != nil || ok {
		t.Fatalf("вид не меняется: %v %v", ok, err)
	}
	if ok, err := s.UpdateWorkspaceDoc(a.ID, upd); err != nil || !ok {
		t.Fatalf("правка: %v %v", ok, err)
	}
	if l, _ := s.GetWorkspaceDocs(a.ID, "card"); len(l) != 1 || l[0].Day != "2026-11-11" || !strings.Contains(string(l[0].Data), `"price":99`) {
		t.Fatalf("после правки: %+v", l)
	}

	// удаление только в своём деле; удаление дела уносит документы
	if err := s.DeleteWorkspaceDoc(b.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.GetWorkspaceDocs(a.ID, "card"); len(l) != 1 {
		t.Fatalf("чужое удаление не трогает: %+v", l)
	}
	if err := s.DeleteWorkspace(a.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.GetWorkspaceDocs(a.ID, ""); len(l) != 0 {
		t.Fatalf("документы ушли с делом: %+v", l)
	}
	if l, _ := s.GetWorkspaceDocs(b.ID, ""); len(l) != 1 {
		t.Fatalf("чужие документы целы: %+v", l)
	}
}

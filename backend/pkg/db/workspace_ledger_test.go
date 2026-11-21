package db

import (
	"errors"
	"testing"
)

func TestWorkspaceLedgerAccountsMarksWeeks(t *testing.T) {
	s, _, _ := newS(t)
	defer s.Close()
	a, _ := s.CreateWorkspace(Workspace{Name: "Glovo"})
	b, _ := s.CreateWorkspace(Workspace{Name: "Бюджет"})

	// счета: замена целиком, повторы и пустые id отбрасываются
	if err := s.SetWorkspaceAccounts(a.ID, []string{"acc2", "acc1", "acc1", ""}); err != nil {
		t.Fatal(err)
	}
	if ids, _ := s.GetWorkspaceAccounts(a.ID); len(ids) != 2 || ids[0] != "acc1" || ids[1] != "acc2" {
		t.Fatalf("счета дела: %v", ids)
	}
	if err := s.SetWorkspaceAccounts(a.ID, []string{"acc3"}); err != nil {
		t.Fatal(err)
	}
	if ids, _ := s.GetWorkspaceAccounts(a.ID); len(ids) != 1 || ids[0] != "acc3" {
		t.Fatalf("замена списка: %v", ids)
	}
	if ids, err := s.GetWorkspaceAccounts("none"); err != nil || ids == nil || len(ids) != 0 {
		t.Fatalf("пусто — [], не nil: %v %v", ids, err)
	}
	if err := s.SetWorkspaceAccounts("", nil); !errors.Is(err, ErrWorkspace) {
		t.Fatalf("без id дела — ErrWorkspace: %v", err)
	}

	// бейджи: операция принадлежит одному делу, пустой бейдж снимает
	if err := s.SetWorkspaceMark(a.ID, "tx1", "income"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetWorkspaceMark(a.ID, "tx1", "payout"); err != nil {
		t.Fatal(err)
	}
	ms, _ := s.GetWorkspaceMarks(a.ID)
	if len(ms) != 1 || ms[0].Badge != "payout" {
		t.Fatalf("повторная пометка меняет бейдж: %+v", ms)
	}
	if err := s.SetWorkspaceMark(b.ID, "tx1", "expense"); err != nil {
		t.Fatal(err)
	}
	if ms, _ = s.GetWorkspaceMarks(a.ID); len(ms) != 0 {
		t.Fatalf("операция переехала в другое дело: %+v", ms)
	}
	if ms, _ = s.GetWorkspaceMarks(b.ID); len(ms) != 1 || ms[0].Badge != "expense" {
		t.Fatalf("и стала его: %+v", ms)
	}
	if err := s.SetWorkspaceMark(a.ID, "tx1", ""); err != nil {
		t.Fatal(err)
	}
	if ms, _ = s.GetWorkspaceMarks(b.ID); len(ms) != 1 {
		t.Fatal("снятие бейджа из чужого дела не должно трогать пометку")
	}
	if err := s.SetWorkspaceMark(b.ID, "tx1", ""); err != nil {
		t.Fatal(err)
	}
	if ms, _ = s.GetWorkspaceMarks(b.ID); len(ms) != 0 {
		t.Fatalf("пометка снята: %+v", ms)
	}
	for _, bad := range []string{"Доход", "Income", "a b", "toolongbadgekey-xx"} {
		if err := s.SetWorkspaceMark(a.ID, "tx2", bad); !errors.Is(err, ErrWorkspace) {
			t.Fatalf("бейдж %q должен отклоняться: %v", bad, err)
		}
	}

	// недели: только понедельники, повтор заменяет, порядок по дате
	if err := s.SetWorkspaceWeek(a.ID, WorkspaceWeek{Week: "2026-11-16", Nominal: 2200}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetWorkspaceWeek(a.ID, WorkspaceWeek{Week: "2026-11-09", Nominal: 1500}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetWorkspaceWeek(a.ID, WorkspaceWeek{Week: "2026-11-16", Nominal: 2300}); err != nil {
		t.Fatal(err)
	}
	ws, _ := s.GetWorkspaceWeeks(a.ID)
	if len(ws) != 2 || ws[0].Week != "2026-11-09" || ws[1].Nominal != 2300 {
		t.Fatalf("недели: %+v", ws)
	}
	for _, bad := range []WorkspaceWeek{{Week: "2026-11-17", Nominal: 1}, {Week: "вчера", Nominal: 1}, {Week: "2026-11-16", Nominal: -1}, {Week: "2026-11-16", Nominal: 1e12}} {
		if err := s.SetWorkspaceWeek(a.ID, bad); !errors.Is(err, ErrWorkspace) {
			t.Fatalf("неделя %+v должна отклоняться: %v", bad, err)
		}
	}
	if err := s.DeleteWorkspaceWeek(a.ID, "2026-11-09"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspaceWeek(a.ID, "2026-01-05"); err != nil {
		t.Fatalf("удалить несуществующую — не ошибка: %v", err)
	}
	if ws, _ = s.GetWorkspaceWeeks(a.ID); len(ws) != 1 {
		t.Fatalf("после удаления: %+v", ws)
	}

	// параметры: ключ и значение проверяются, повтор заменяет, удаление возвращает к умолчанию
	if err := s.SetWorkspaceParam(a.ID, WorkspaceParam{Key: "tax_rate", Value: 5}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetWorkspaceParam(a.ID, WorkspaceParam{Key: "tax_rate", Value: 6}); err != nil {
		t.Fatal(err)
	}
	if ps, _ := s.GetWorkspaceParams(a.ID); len(ps) != 1 || ps[0].Value != 6 {
		t.Fatalf("параметры дела: %v", ps)
	}
	if err := s.SetWorkspaceParam(a.ID, WorkspaceParam{Key: "Bad", Value: 1}); !errors.Is(err, ErrWorkspace) {
		t.Fatalf("плохой ключ: %v", err)
	}
	if err := s.SetWorkspaceParam(a.ID, WorkspaceParam{Key: "esv", Value: -1}); !errors.Is(err, ErrWorkspace) {
		t.Fatalf("отрицательное значение: %v", err)
	}
	if err := s.SetWorkspaceParam(b.ID, WorkspaceParam{Key: "esv", Value: 1902.34}); err != nil {
		t.Fatal(err)
	}

	// удаление дела уносит его учёт, чужое не трогает
	if err := s.SetWorkspaceMark(a.ID, "tx9", "tips"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetWorkspaceMark(b.ID, "tx8", "loss"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspace(a.ID); err != nil {
		t.Fatal(err)
	}
	if ids, _ := s.GetWorkspaceAccounts(a.ID); len(ids) != 0 {
		t.Fatalf("счета ушли с делом: %v", ids)
	}
	if ms, _ := s.GetWorkspaceMarks(a.ID); len(ms) != 0 {
		t.Fatalf("пометки ушли с делом: %v", ms)
	}
	if ws, _ := s.GetWorkspaceWeeks(a.ID); len(ws) != 0 {
		t.Fatalf("недели ушли с делом: %v", ws)
	}
	if ps, _ := s.GetWorkspaceParams(a.ID); len(ps) != 0 {
		t.Fatalf("параметры ушли с делом: %v", ps)
	}
	if ms, _ := s.GetWorkspaceMarks(b.ID); len(ms) != 1 {
		t.Fatalf("чужое дело не тронуто: %v", ms)
	}
	if ps, _ := s.GetWorkspaceParams(b.ID); len(ps) != 1 {
		t.Fatalf("чужие параметры не тронуты: %v", ps)
	}
}

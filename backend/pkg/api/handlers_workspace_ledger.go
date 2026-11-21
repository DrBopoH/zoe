package api

import (
	"net/http"

	"economic/pkg/db"
)

// Учёт дела: /api/workspaces/accounts, /marks, /weeks. Во всех GET и DELETE дело задаётся ?id=, в PUT — полем workspace_id.

// GET ?id= → ["acc", …]; PUT {workspace_id, accounts: […]} заменяет список целиком.
func (s *Server) HandleWorkspaceAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.GetWorkspaceAccounts(r.URL.Query().Get("id"))
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, list)
	case http.MethodPut:
		var in struct {
			WorkspaceID string   `json:"workspace_id"`
			Accounts    []string `json:"accounts"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		if workspaceFail(w, s.storage.SetWorkspaceAccounts(in.WorkspaceID, in.Accounts)) {
			return
		}
		writeOK(w)
	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// GET ?id= → [{tx_id, workspace_id, badge}]; PUT {workspace_id, tx_id, badge} ставит бейдж (пустой — снимает).
func (s *Server) HandleWorkspaceMarks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.GetWorkspaceMarks(r.URL.Query().Get("id"))
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, list)
	case http.MethodPut:
		var in db.WorkspaceMark
		if !readJSON(w, r, &in) {
			return
		}
		if workspaceFail(w, s.storage.SetWorkspaceMark(in.WorkspaceID, in.TxID, in.Badge)) {
			return
		}
		writeOK(w)
	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// GET ?id= → [{week, nominal}]; PUT {workspace_id, week, nominal}; DELETE ?id=&week=.
func (s *Server) HandleWorkspaceWeeks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.GetWorkspaceWeeks(r.URL.Query().Get("id"))
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, list)
	case http.MethodPut:
		var in struct {
			WorkspaceID string `json:"workspace_id"`
			db.WorkspaceWeek
		}
		if !readJSON(w, r, &in) {
			return
		}
		if workspaceFail(w, s.storage.SetWorkspaceWeek(in.WorkspaceID, in.WorkspaceWeek)) {
			return
		}
		writeOK(w)
	case http.MethodDelete:
		q := r.URL.Query()
		if q.Get("id") == "" || q.Get("week") == "" {
			fail(w, http.StatusBadRequest, "Missing id or week")
			return
		}
		if err := s.storage.DeleteWorkspaceWeek(q.Get("id"), q.Get("week")); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)
	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

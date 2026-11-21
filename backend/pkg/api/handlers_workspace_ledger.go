package api

import (
	"net/http"

	"economic/pkg/db"
)

// Учёт дела: /api/workspaces/accounts, /marks, /weeks, /params, /entries, /docs. Во всех GET и DELETE дело задаётся ?id=, в PUT — полем workspace_id.

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
		q := r.URL.Query()
		get := s.storage.GetWorkspaceMarks
		if q.Get("other") == "1" { // пометки соседних дел
			get = s.storage.GetOtherWorkspaceMarks
		}
		list, err := get(q.Get("id"))
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

// GET ?id= → [{key, value}]; PUT {workspace_id, key, value}; DELETE ?id=&key=.
func (s *Server) HandleWorkspaceParams(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.GetWorkspaceParams(r.URL.Query().Get("id"))
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, list)
	case http.MethodPut:
		var in struct {
			WorkspaceID string `json:"workspace_id"`
			db.WorkspaceParam
		}
		if !readJSON(w, r, &in) {
			return
		}
		if workspaceFail(w, s.storage.SetWorkspaceParam(in.WorkspaceID, in.WorkspaceParam)) {
			return
		}
		writeOK(w)
	case http.MethodDelete:
		q := r.URL.Query()
		if q.Get("id") == "" || q.Get("key") == "" {
			fail(w, http.StatusBadRequest, "Missing id or key")
			return
		}
		if err := s.storage.DeleteWorkspaceParam(q.Get("id"), q.Get("key")); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)
	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// GET ?id= → [{id, day, badge, amount, note, tx_id}]; POST {workspace_id, day, badge, amount, note, tx_id} → запись с id;
// PUT {workspace_id, id, day, badge, amount, note, tx_id}; DELETE ?id=&entry=.
func (s *Server) HandleWorkspaceEntries(w http.ResponseWriter, r *http.Request) {
	type body struct {
		WorkspaceID string `json:"workspace_id"`
		db.WorkspaceEntry
	}
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.GetWorkspaceEntries(r.URL.Query().Get("id"))
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, list)
	case http.MethodPost:
		var in body
		if !readJSON(w, r, &in) {
			return
		}
		out, err := s.storage.CreateWorkspaceEntry(in.WorkspaceID, in.WorkspaceEntry)
		if workspaceFail(w, err) {
			return
		}
		writeJSON(w, out)
	case http.MethodPut:
		var in body
		if !readJSON(w, r, &in) {
			return
		}
		ok, err := s.storage.UpdateWorkspaceEntry(in.WorkspaceID, in.WorkspaceEntry)
		if workspaceFail(w, err) {
			return
		}
		if !ok {
			fail(w, http.StatusNotFound, "No such entry")
			return
		}
		writeOK(w)
	case http.MethodDelete:
		q := r.URL.Query()
		if q.Get("id") == "" || q.Get("entry") == "" {
			fail(w, http.StatusBadRequest, "Missing id or entry")
			return
		}
		if err := s.storage.DeleteWorkspaceEntry(q.Get("id"), q.Get("entry")); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)
	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// GET ?id=[&kind=] → [{id, kind, day, data}]; POST {workspace_id, kind, day, data} → документ с id;
// PUT {workspace_id, id, kind, day, data}; DELETE ?id=&doc=.
func (s *Server) HandleWorkspaceDocs(w http.ResponseWriter, r *http.Request) {
	type body struct {
		WorkspaceID string `json:"workspace_id"`
		db.WorkspaceDoc
	}
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		list, err := s.storage.GetWorkspaceDocs(q.Get("id"), q.Get("kind"))
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, list)
	case http.MethodPost:
		var in body
		if !readJSON(w, r, &in) {
			return
		}
		out, err := s.storage.CreateWorkspaceDoc(in.WorkspaceID, in.WorkspaceDoc)
		if workspaceFail(w, err) {
			return
		}
		writeJSON(w, out)
	case http.MethodPut:
		var in body
		if !readJSON(w, r, &in) {
			return
		}
		ok, err := s.storage.UpdateWorkspaceDoc(in.WorkspaceID, in.WorkspaceDoc)
		if workspaceFail(w, err) {
			return
		}
		if !ok {
			fail(w, http.StatusNotFound, "No such doc")
			return
		}
		writeOK(w)
	case http.MethodDelete:
		q := r.URL.Query()
		if q.Get("id") == "" || q.Get("doc") == "" {
			fail(w, http.StatusBadRequest, "Missing id or doc")
			return
		}
		if err := s.storage.DeleteWorkspaceDoc(q.Get("id"), q.Get("doc")); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)
	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

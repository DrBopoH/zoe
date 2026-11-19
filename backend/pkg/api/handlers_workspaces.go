package api

import (
	"errors"
	"net/http"

	"economic/pkg/db"
)

// Дела (рабочие пространства, Core DB): список, создание, правка и удаление. Обложка — цвет и/или маленькая картинка.

func (s *Server) HandleWorkspaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.storage.GetWorkspaces()
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, list)

	case http.MethodPost:
		var in db.Workspace
		if !readJSON(w, r, &in) {
			return
		}
		in.ID, in.CreatedAt = "", 0 // id и время ставит сервер
		out, err := s.storage.CreateWorkspace(in)
		if workspaceFail(w, err) {
			return
		}
		writeJSON(w, out)

	case http.MethodPut:
		var in db.Workspace
		if !readJSON(w, r, &in) {
			return
		}
		ok, err := s.storage.UpdateWorkspace(in)
		if workspaceFail(w, err) {
			return
		}
		if !ok {
			fail(w, http.StatusNotFound, "No such workspace")
			return
		}
		writeOK(w)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			fail(w, http.StatusBadRequest, "Missing id")
			return
		}
		if err := s.storage.DeleteWorkspace(id); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)

	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// workspaceFail отвечает на ошибку и возвращает true: неверные поля — 400 с текстом, остальное — сбой сервера.
func workspaceFail(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, db.ErrWorkspace):
		fail(w, http.StatusBadRequest, err.Error())
	default:
		failErr(w, err)
	}
	return true
}

package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"economic/pkg/db"
)

// Лица, алиасы, связи счетов и скрытые виды IBAN (Core DB).

func (s *Server) HandleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users, err := s.storage.GetUsers()
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, users)

	case http.MethodPost:
		var u db.User
		if !readJSON(w, r, &u) {
			return
		}
		if strings.TrimSpace(u.Name) == "" {
			fail(w, http.StatusBadRequest, "Name is required")
			return
		}
		if u.ID == "" {
			u.ID = fmt.Sprintf("user_%d", time.Now().UnixNano())
		}
		if err := s.storage.CreateUser(u); err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, u)

	case http.MethodPut:
		var u db.User
		if !readJSON(w, r, &u) {
			return
		}
		if err := s.storage.UpdateUser(u); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			fail(w, http.StatusBadRequest, "Missing id")
			return
		}
		if err := s.storage.DeleteUser(id); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)

	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

type AliasRequest struct {
	UserID string `json:"user_id"`
	Alias  string `json:"alias"`
}

func (s *Server) HandleAddAlias(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req AliasRequest
		if !readJSON(w, r, &req) {
			return
		}
		if err := s.storage.AddUserAlias(req.UserID, req.Alias); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)

	case http.MethodDelete:
		alias := r.URL.Query().Get("alias")
		if alias == "" {
			fail(w, http.StatusBadRequest, "Missing alias")
			return
		}
		if err := s.storage.RemoveUserAlias(alias); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)

	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

type LinkIBANRequest struct {
	UserID            string `json:"user_id"`
	AccountIdentifier string `json:"account_identifier"`
	ParentIdentifier  string `json:"parent_identifier"`
	Note              string `json:"note"`
}

func (s *Server) HandleLinkAccount(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req LinkIBANRequest
		if !readJSON(w, r, &req) {
			return
		}
		// Note — это название алиаса, к которому относится связь
		if err := s.storage.LinkAccount(req.UserID, req.Note, req.AccountIdentifier, req.ParentIdentifier); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)

	case http.MethodDelete:
		identifier := r.URL.Query().Get("identifier")
		if identifier == "" {
			fail(w, http.StatusBadRequest, "Missing identifier")
			return
		}
		// alias обязателен для корректного теневого бана; без него — убрать у всех алиасов
		alias := r.URL.Query().Get("alias")
		if err := s.storage.UnlinkAccount(alias, identifier); err != nil {
			failErr(w, err)
			return
		}
		writeOK(w)

	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// GET /api/alias-structures — запомненные структуры алиасов (в т.ч. бесхозных, user_id == "").
func (s *Server) HandleAliasStructures(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	list, err := s.storage.GetAliasStructures()
	if err != nil {
		failErr(w, err)
		return
	}
	writeJSON(w, list)
}

// HandleIBANMasks — GET /api/iban-masks: скрытые виды IBAN (нет записи — генерируется и сохраняется);
// POST/PUT {iban, masked}: свой псевдоним-имя счёта, пустой masked — вернуть автоматическую маску.
func (s *Server) HandleIBANMasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		m, err := s.storage.IBANMasks()
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, m)
	case http.MethodPost, http.MethodPut:
		var req struct {
			IBAN   string `json:"iban"`
			Masked string `json:"masked"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.IBAN) == "" {
			fail(w, http.StatusBadRequest, "Invalid payload")
			return
		}
		m, err := s.storage.SetIBANMask(req.IBAN, req.Masked)
		if err != nil {
			failErr(w, err)
			return
		}
		writeJSON(w, m)
	default:
		fail(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

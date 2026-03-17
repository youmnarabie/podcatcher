// internal/api/rules.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.ListRules(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, rules)
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pattern  string `json:"pattern"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Pattern == "" {
		writeError(w, 400, "pattern required")
		return
	}
	rule, err := s.store.CreateRule(r.Context(), chi.URLParam(r, "id"), body.Pattern, body.Priority)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, rule)
}

func (s *Server) updateRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pattern  string `json:"pattern"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if err := s.store.UpdateRule(r.Context(), chi.URLParam(r, "id"), body.Pattern, body.Priority); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteRule(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

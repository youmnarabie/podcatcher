// internal/api/series.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listSeries(w http.ResponseWriter, r *http.Request) {
	series, err := s.store.ListSeries(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, series)
}

func (s *Server) createSeries(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, 400, "name required")
		return
	}
	ser, err := s.store.UpsertSeries(r.Context(), chi.URLParam(r, "id"), body.Name)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, ser)
}

func (s *Server) renameSeries(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, 400, "name required")
		return
	}
	if err := s.store.RenameSeries(r.Context(), chi.URLParam(r, "id"), body.Name); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

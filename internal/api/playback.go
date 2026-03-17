// internal/api/playback.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) getPlayback(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetPlayback(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		// No state yet — return zero state
		writeJSON(w, 200, map[string]any{"position_seconds": 0, "completed": false})
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) upsertPlayback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PositionSeconds int  `json:"position_seconds"`
		Completed       bool `json:"completed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	p, err := s.store.UpsertPlayback(r.Context(), chi.URLParam(r, "id"), body.PositionSeconds, body.Completed)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, p)
}

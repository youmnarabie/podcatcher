// internal/api/feeds.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := s.store.ListFeeds(r.Context())
	if err != nil {
		writeError(w, 500, "internal error")
		return
	}
	writeJSON(w, 200, feeds)
}

func (s *Server) createFeed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		writeError(w, 400, "url required")
		return
	}
	feed, err := s.store.CreateFeed(r.Context(), body.URL)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	go func() { _ = s.ingester.FetchAndIngest(r.Context(), feed.ID, feed.URL) }()
	writeJSON(w, 201, feed)
}

func (s *Server) deleteFeed(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteFeed(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) refreshFeed(w http.ResponseWriter, r *http.Request) {
	feed, err := s.store.GetFeed(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 404, "feed not found")
		return
	}
	go func() { _ = s.ingester.FetchAndIngest(r.Context(), feed.ID, feed.URL) }()
	writeJSON(w, 202, map[string]string{"status": "refreshing"})
}

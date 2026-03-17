// internal/api/episodes.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/youmnarabie/poo/internal/store"
)

func (s *Server) listEpisodes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.EpisodeFilter{
		FeedID:   q.Get("feed_id"),
		SeriesID: q.Get("series_id"),
		Sort:     q.Get("sort"),
		Order:    q.Get("order"),
	}
	if played := q.Get("played"); played == "true" {
		t := true; f.Played = &t
	} else if played == "false" {
		v := false; f.Played = &v
	}
	if df := q.Get("date_from"); df != "" {
		if t, err := time.Parse(time.RFC3339, df); err == nil {
			f.DateFrom = &t
		}
	}
	if dt := q.Get("date_to"); dt != "" {
		if t, err := time.Parse(time.RFC3339, dt); err == nil {
			f.DateTo = &t
		}
	}
	if l, _ := strconv.Atoi(q.Get("limit")); l > 0 {
		f.Limit = l
	}
	f.Offset, _ = strconv.Atoi(q.Get("offset"))

	eps, err := s.store.ListEpisodes(r.Context(), f)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, eps)
}

func (s *Server) getEpisode(w http.ResponseWriter, r *http.Request) {
	ep, err := s.store.GetEpisode(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 404, "not found")
		return
	}
	writeJSON(w, 200, ep)
}

func (s *Server) addEpisodeSeries(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SeriesID      string `json:"series_id"`
		EpisodeNumber *int   `json:"episode_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SeriesID == "" {
		writeError(w, 400, "series_id required")
		return
	}
	err := s.store.AssignEpisodeToSeries(r.Context(),
		chi.URLParam(r, "id"), body.SeriesID, body.EpisodeNumber, true)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) removeEpisodeSeries(w http.ResponseWriter, r *http.Request) {
	err := s.store.RemoveEpisodeFromSeries(r.Context(),
		chi.URLParam(r, "id"), chi.URLParam(r, "seriesID"))
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

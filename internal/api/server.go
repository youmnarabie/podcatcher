// internal/api/server.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/store"
)

type Server struct {
	store    *store.Store
	ingester *ingester.Ingester
	router   *chi.Mux
}

func New(s *store.Store, ing *ingester.Ingester) *Server {
	srv := &Server{store: s, ingester: ing}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/feeds", srv.listFeeds)
		r.Post("/feeds", srv.createFeed)
		r.Delete("/feeds/{id}", srv.deleteFeed)
		r.Post("/feeds/{id}/refresh", srv.refreshFeed)
		r.Get("/feeds/{id}/series", srv.listSeries)
		r.Post("/feeds/{id}/series", srv.createSeries)
		r.Get("/feeds/{id}/rules", srv.listRules)
		r.Post("/feeds/{id}/rules", srv.createRule)

		r.Get("/episodes", srv.listEpisodes)
		r.Get("/episodes/{id}", srv.getEpisode)
		r.Get("/episodes/{id}/playback", srv.getPlayback)
		r.Put("/episodes/{id}/playback", srv.upsertPlayback)
		r.Post("/episodes/{id}/series", srv.addEpisodeSeries)
		r.Delete("/episodes/{id}/series/{seriesID}", srv.removeEpisodeSeries)

		r.Patch("/series/{id}", srv.renameSeries)
		r.Patch("/rules/{id}", srv.updateRule)
		r.Delete("/rules/{id}", srv.deleteRule)

		r.Post("/opml/import", srv.opmlImport)
		r.Get("/opml/export", srv.opmlExport)

		r.Get("/search", srv.search)
	})

	srv.router = r
	return srv
}

func (s *Server) Handler() http.Handler { return s.router }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

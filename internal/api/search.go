// internal/api/search.go
package api

import "net/http"

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	result, err := s.store.Search(r.Context(), q)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

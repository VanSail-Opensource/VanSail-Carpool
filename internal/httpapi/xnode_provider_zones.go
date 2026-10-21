package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) xnodeProviderZones(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	value, err := s.deps.XNode.ProviderZones(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

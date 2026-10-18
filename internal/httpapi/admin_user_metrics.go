package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
)

func (s *Server) adminUserMetrics(w http.ResponseWriter, r *http.Request) {
	if s.deps.XNodeMaster == nil {
		s.xnodeError(w, r, xnode.ErrUnavailable)
		return
	}
	value, err := s.deps.XNodeMaster.ProfileMetrics(r.Context(), chiURLParam(r, "userId"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, value)
}

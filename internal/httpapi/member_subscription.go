package httpapi

import (
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

func (s *Server) memberConnectivitySummary(w http.ResponseWriter, r *http.Request) {
	s.memberSubscriptionResponse(w, r, false)
}
func (s *Server) memberSubscription(w http.ResponseWriter, r *http.Request) {
	s.memberSubscriptionResponse(w, r, true)
}

func (s *Server) memberSubscriptionResponse(w http.ResponseWriter, r *http.Request, links bool) {
	w.Header().Set("Cache-Control", "no-store")
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	result, err := s.deps.Catalog.Subscription(r.Context(), user, links)
	if err != nil {
		s.writeError(w, r, http.StatusBadGateway, connectivity.ErrorCode(err), "Subscription connectivity is temporarily unavailable.")
		return
	}
	if links {
		writeJSON(w, http.StatusOK, result)
	} else {
		writeJSON(w, http.StatusOK, result.Summary)
	}
}

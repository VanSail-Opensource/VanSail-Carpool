package httpapi

import "net/http"

func (s *Server) memberSubscription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	result, err := s.deps.Catalog.Subscription(r.Context(), user, true)
	if err != nil {
		s.writeError(w, r, http.StatusBadGateway, "SUBSCRIPTION_UNAVAILABLE", "Subscription is temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

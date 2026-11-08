package httpapi

import "net/http"

func (s *Server) ownedUsageBreakdown(w http.ResponseWriter, r *http.Request) {
	privateSubscriptionHeaders(w)
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	result, err := s.deps.XNode.OwnedUsageBreakdown(r.Context(), user.ID)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

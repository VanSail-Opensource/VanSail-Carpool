package httpapi

import "net/http"

func (s *Server) ownedPAYGHistory(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	result, err := s.deps.XNode.OwnedPAYGHistory(r.Context(), user.ID)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, result)
}

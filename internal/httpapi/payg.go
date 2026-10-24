package httpapi

import "net/http"

func (s *Server) ownedPAYG(w http.ResponseWriter, r *http.Request) {
	privateSubscriptionHeaders(w)
	value, err := s.deps.XNode.OwnedPAYG(r.Context(), currentUser(r).ID)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

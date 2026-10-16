package httpapi

import "net/http"

func (s *Server) xnodeServiceSubscription(w http.ResponseWriter, r *http.Request) {
	privateSubscriptionHeaders(w)
	metadata, err := s.deps.XNode.ServiceSubscription(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, metadata)
}

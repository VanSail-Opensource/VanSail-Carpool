package httpapi

import "net/http"

// adminRequestUserConnections starts the existing durable scan for the profile target.
func (s *Server) adminRequestUserConnections(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	user, err := s.deps.Store.UserByID(r.Context(), chiURLParam(r, "userId"))
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	scan, err := s.deps.Connections.RequestAdmin(r.Context(), user, key)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, scan)
}

// adminPollUserConnections scopes an ephemeral scan projection to its profile target.
func (s *Server) adminPollUserConnections(w http.ResponseWriter, r *http.Request) {
	scan, err := s.deps.Connections.PollAdmin(r.Context(), chiURLParam(r, "userId"), chiURLParam(r, "scanId"))
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, scan)
}

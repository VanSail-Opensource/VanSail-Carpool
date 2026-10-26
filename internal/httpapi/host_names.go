package httpapi

import "net/http"

func (s *Server) saveOwnedHostName(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	var input struct {
		Name *string `json:"name"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.Name == nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Enter a host name or an empty value to restore its published name.")
		return
	}
	result, err := s.deps.XNode.SaveOwnedHostName(r.Context(), user.ID, chiURLParam(r, "id"), *input.Name)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, result)
}

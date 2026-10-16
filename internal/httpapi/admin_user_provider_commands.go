package httpapi

import "net/http"

func (s *Server) adminTemporaryBan(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		DurationMinutes int    `json:"durationMinutes"`
		Reason          string `json:"reason"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_TEMPORARY_BAN", "Duration and reason are required.")
		return
	}
	receipt, err := s.deps.AdminUsers.TemporaryBan(r.Context(), currentUser(r).ID, chiURLParam(r, "userId"), key, request.Reason, request.DurationMinutes)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) adminTemporaryUnban(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_TEMPORARY_UNBAN", "A reason is required.")
		return
	}
	receipt, err := s.deps.AdminUsers.TemporaryUnban(r.Context(), currentUser(r).ID, chiURLParam(r, "userId"), key, request.Reason)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

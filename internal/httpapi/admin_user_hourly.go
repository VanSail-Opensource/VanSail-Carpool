package httpapi

import "net/http"

// adminUserHourly returns the owner-scoped hourly projection for one member.
func (s *Server) adminUserHourly(w http.ResponseWriter, r *http.Request) {
	view, err := s.deps.Admin.AdminHourly(r.Context(), chiURLParam(r, "userId"))
	if err != nil {
		s.hourlyAdminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// adminDisableUserHourly stops one enrollment as an administrator without the
// member's voluntary cooldown. The paid hour is retained.
func (s *Server) adminDisableUserHourly(w http.ResponseWriter, r *http.Request) {
	if s.deps.Hourly == nil {
		s.writeError(w, r, http.StatusServiceUnavailable, "HOURLY_UNAVAILABLE", "Hourly access is temporarily unavailable.")
		return
	}
	key, ok := s.requireHourlyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		PolicyVersion string `json:"policyVersion"`
	}
	id := chiURLParam(r, "enrollmentId")
	if err := decodeJSON(w, r, &body); err != nil || !hourlyPolicyVersion(body.PolicyVersion) || id == "" || len(id) > 128 {
		s.hourlyInputInvalid(w, r)
		return
	}
	receipt, err := s.deps.Hourly.DisableForAdmin(r.Context(), currentUser(r).ID,
		chiURLParam(r, "userId"), id, body.PolicyVersion, key)
	if err != nil {
		s.hourlyMemberFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

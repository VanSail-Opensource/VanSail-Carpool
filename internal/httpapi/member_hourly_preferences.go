package httpapi

import "net/http"

func (s *Server) memberHourlyExtra(w http.ResponseWriter, r *http.Request) {
	if !s.requireHourlyMember(w, r) {
		return
	}
	var body struct {
		Enabled               *bool   `json:"enabled"`
		PolicyVersion         string  `json:"policyVersion"`
		AcceptedPriceTXBMinor *string `json:"acceptedPriceTXBMinor"`
	}
	if err := decodeJSON(w, r, &body); err != nil || body.Enabled == nil {
		s.hourlyInputInvalid(w, r)
		return
	}
	var cap *int64
	if body.AcceptedPriceTXBMinor != nil {
		value, ok := hourlyDecimal(*body.AcceptedPriceTXBMinor, *body.Enabled)
		if !ok {
			s.hourlyInputInvalid(w, r)
			return
		}
		cap = &value
	}
	if *body.Enabled && (!hourlyPolicyVersion(body.PolicyVersion) || cap == nil) {
		s.hourlyInputInvalid(w, r)
		return
	}
	value, err := s.deps.Hourly.SetExtraPreference(r.Context(), currentUser(r), *body.Enabled, body.PolicyVersion, cap)
	if err != nil {
		s.hourlyMemberFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

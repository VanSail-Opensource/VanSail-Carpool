package httpapi

import (
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourlybilling"
)

func (s *Server) requireHourlyMember(w http.ResponseWriter, r *http.Request) bool {
	if !s.requireOnboarded(w, r, currentUser(r)) {
		return false
	}
	if s.deps.Hourly == nil {
		s.writeError(w, r, http.StatusServiceUnavailable, "HOURLY_UNAVAILABLE", "Hourly access is temporarily unavailable.")
		return false
	}
	return true
}

func (s *Server) memberHourly(w http.ResponseWriter, r *http.Request) {
	if !s.requireHourlyMember(w, r) {
		return
	}
	view, err := s.deps.Hourly.View(r.Context(), currentUser(r))
	if err != nil {
		s.hourlyMemberFailure(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) memberHourlyActivate(w http.ResponseWriter, r *http.Request) {
	if !s.requireHourlyMember(w, r) {
		return
	}
	key, ok := s.requireHourlyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		GroupID        string `json:"groupId"`
		PolicyVersion  string `json:"policyVersion"`
		OfferRevision  string `json:"offerRevision"`
		ActivationCode string `json:"activationCode,omitempty"`
	}
	if err := decodeJSON(w, r, &body); err != nil || body.GroupID == "" || strings.TrimSpace(body.GroupID) != body.GroupID || len(body.GroupID) > 128 || !hourlyPolicyVersion(body.PolicyVersion) || len(body.ActivationCode) > 1024 {
		s.hourlyInputInvalid(w, r)
		return
	}
	revision, ok := hourlyDecimal(body.OfferRevision, true)
	if !ok {
		s.hourlyInputInvalid(w, r)
		return
	}
	receipt, err := s.deps.Hourly.Activate(r.Context(), currentUser(r), hourlybilling.ActivationRequest{
		GroupID: body.GroupID, PolicyVersion: body.PolicyVersion, OfferRevision: revision, ActivationCode: body.ActivationCode}, key)
	if err != nil {
		s.hourlyMemberFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) memberHourlyDisable(w http.ResponseWriter, r *http.Request) {
	if !s.requireHourlyMember(w, r) {
		return
	}
	key, ok := s.requireHourlyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		PolicyVersion string `json:"policyVersion"`
	}
	id := chiURLParam(r, "id")
	if err := decodeJSON(w, r, &body); err != nil || !hourlyPolicyVersion(body.PolicyVersion) || id == "" || len(id) > 128 {
		s.hourlyInputInvalid(w, r)
		return
	}
	receipt, err := s.deps.Hourly.Disable(r.Context(), currentUser(r), id, body.PolicyVersion, key)
	if err != nil {
		s.hourlyMemberFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

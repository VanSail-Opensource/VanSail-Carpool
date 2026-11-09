package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) adminHourlyPolicy(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.deps.Admin.HourlyPolicy(r.Context())
	if err != nil {
		s.hourlyAdminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) adminSaveHourlyPolicy(w http.ResponseWriter, r *http.Request) {
	var input adminHourlyPolicyInput
	if err := decodeJSON(w, r, &input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "HOURLY_POLICY_INVALID", "Hourly policy fields are invalid.")
		return
	}
	value, complete := input.snapshot()
	if !complete {
		s.writeError(w, r, http.StatusBadRequest, "HOURLY_POLICY_INVALID", "Provide every hourly policy field.")
		return
	}
	snapshot, err := s.deps.Admin.SaveHourlyPolicy(r.Context(), currentUser(r).ID, value.Version, value.Policy)
	if err != nil {
		s.hourlyAdminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) adminHourlyOffers(w http.ResponseWriter, r *http.Request) {
	offers, err := s.deps.Admin.HourlyOffers(r.Context())
	if err != nil {
		s.hourlyAdminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, offers)
}

func (s *Server) adminSaveHourlyOffer(w http.ResponseWriter, r *http.Request) {
	var input adminHourlyOfferInput
	if err := decodeJSON(w, r, &input); err != nil || !input.complete() {
		s.writeError(w, r, http.StatusBadRequest, "HOURLY_OFFER_INVALID", "Hourly price fields are invalid.")
		return
	}
	offer, err := s.deps.Admin.SaveHourlyOffer(r.Context(), currentUser(r).ID,
		model.HourlyOffer{GroupID: chiURLParam(r, "groupId"), Enabled: *input.Enabled,
			HourlyPriceTXBMinor: *input.HourlyPriceTXBMinor, RequiredHours: *input.RequiredHours}, *input.Revision)
	if err != nil {
		s.hourlyAdminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, offer)
}

func (s *Server) hourlyAdminFailure(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrConflict):
		s.writeError(w, r, http.StatusConflict, "HOURLY_TERMS_CHANGED", "Reload the current hourly terms before saving.")
	case errors.Is(err, model.ErrHourlyPolicyInvalid):
		s.writeError(w, r, http.StatusUnprocessableEntity, "HOURLY_POLICY_INVALID", "Hourly units or price limits are invalid.")
	case errors.Is(err, admin.ErrHourlyPolicyUnavailable):
		s.writeError(w, r, http.StatusServiceUnavailable, "HOURLY_UNAVAILABLE", "Hourly configuration is unavailable.")
	default:
		s.adminFailure(w, r, err)
	}
}

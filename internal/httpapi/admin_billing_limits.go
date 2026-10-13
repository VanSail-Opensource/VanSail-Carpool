package httpapi

import (
	"net/http"
	"strconv"
	"time"
)

type billingAmountLimitsRequest struct {
	MinimumTXBMinor string `json:"minimumTxbMinor"`
	MaximumTXBMinor string `json:"maximumTxbMinor"`
}

func (s *Server) adminBillingAmountLimits(w http.ResponseWriter, r *http.Request) {
	bounds, err := s.deps.Store.AddTXBBounds(r.Context())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, bounds)
}

func (s *Server) adminUpdateBillingAmountLimits(w http.ResponseWriter, r *http.Request) {
	var request billingAmountLimitsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_BILLING_AMOUNT_LIMITS", "Minimum and maximum TXB amounts are required.")
		return
	}
	minimum, minimumErr := strconv.ParseInt(request.MinimumTXBMinor, 10, 64)
	maximum, maximumErr := strconv.ParseInt(request.MaximumTXBMinor, 10, 64)
	if minimumErr != nil || maximumErr != nil || minimum <= 0 || maximum < minimum {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_BILLING_AMOUNT_LIMITS", "Amounts must be positive integer hundredths of TXB, with maximum greater than or equal to minimum.")
		return
	}
	bounds, err := s.deps.Store.UpdateAddTXBBounds(r.Context(), minimum, maximum, currentUser(r).ID, time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, bounds)
}

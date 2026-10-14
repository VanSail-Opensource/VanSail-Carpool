package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"net/http"
)

func (s *Server) adminEntitlementRefundQuote(w http.ResponseWriter, r *http.Request) {
	quote, err := s.deps.AdminUsers.RefundEntitlementQuote(r.Context(), chiURLParam(r, "userId"), chiURLParam(r, "entitlementId"))
	if err != nil {
		if errors.Is(err, admin.ErrRefundUsageUnavailable) {
			s.deps.Logger.Warn("administrator refund usage unavailable", "error", err)
			s.writeError(w, r, http.StatusBadGateway, "REFUND_USAGE_UNAVAILABLE", "Traffic could not be verified. Retry or enter a manual refund amount.")
			return
		}
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

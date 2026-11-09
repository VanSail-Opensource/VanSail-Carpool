package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type adminEntitlementCreateRequest struct {
	Reason            string    `json:"reason"`
	ComboID           string    `json:"comboId"`
	ValidFrom         time.Time `json:"validFrom"`
	ValidUntil        time.Time `json:"validUntil"`
	TrafficLimitBytes string    `json:"trafficLimitBytes"`
	ResetStrategy     string    `json:"resetStrategy"`
	SquadUUIDs        []string  `json:"squadUuids"`
}

// adminCreateEntitlement grants a zero-charge fixed term through one audited
// idempotent administrator command. It never debits the member wallet.
func (s *Server) adminCreateEntitlement(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request adminEntitlementCreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ENTITLEMENT_GRANT", "Provide complete entitlement grant fields.")
		return
	}
	traffic, err := strconv.ParseInt(request.TrafficLimitBytes, 10, 64)
	if err != nil || traffic <= 0 {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_TRAFFIC_LIMIT", "Traffic limit must be a positive integer.")
		return
	}
	purchase, err := s.deps.AdminUsers.CreateEntitlement(r.Context(), currentUser(r).ID,
		chiURLParam(r, "userId"), key, admin.AdminEntitlementCreate{
			Reason: request.Reason, ComboID: request.ComboID, ValidFrom: request.ValidFrom,
			ValidUntil: request.ValidUntil, TrafficLimitBytes: traffic,
			ResetStrategy: request.ResetStrategy, SquadUUIDs: request.SquadUUIDs,
		})
	if err != nil {
		if errors.Is(err, database.ErrEntitlementOverlap) {
			s.writeError(w, r, http.StatusConflict, "ENTITLEMENT_OVERLAP", "The requested term overlaps an existing entitlement.")
			return
		}
		if errors.Is(err, database.ErrHourlyLegacyConflict) {
			s.writeError(w, r, http.StatusConflict, "HOURLY_LEGACY_ACTIVE", "The member has an active hourly enrollment. Stop it before granting a fixed-term plan.")
			return
		}
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, adminEntitlementResponse{Purchase: purchase, UserID: purchase.UserID})
}

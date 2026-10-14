package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
)

type entitlementEditRequest struct {
	Version           string    `json:"version"`
	Reason            string    `json:"reason"`
	ComboID           string    `json:"comboId"`
	ValidFrom         time.Time `json:"validFrom"`
	ValidUntil        time.Time `json:"validUntil"`
	Status            string    `json:"status"`
	TrafficLimitBytes string    `json:"trafficLimitBytes"`
	ResetStrategy     string    `json:"resetStrategy"`
	SquadUUIDs        []string  `json:"squadUuids"`
}

func (s *Server) adminEditEntitlement(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request entitlementEditRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ENTITLEMENT_EDIT", "The complete entitlement fields are required.")
		return
	}
	version, err := time.Parse(time.RFC3339Nano, request.Version)
	if err != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_ENTITLEMENT_VERSION", "Refresh the entitlement before editing it.")
		return
	}
	traffic, err := strconv.ParseInt(request.TrafficLimitBytes, 10, 64)
	if err != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_TRAFFIC_LIMIT", "Traffic limit must be a positive integer.")
		return
	}
	purchase, err := s.deps.AdminUsers.EditEntitlement(r.Context(), currentUser(r).ID,
		chiURLParam(r, "userId"), chiURLParam(r, "entitlementId"), key, admin.EntitlementEdit{
			Version: version, Reason: request.Reason, ComboID: request.ComboID, ValidFrom: request.ValidFrom,
			ValidUntil: request.ValidUntil, Status: request.Status, TrafficLimitBytes: traffic,
			ResetStrategy: request.ResetStrategy, SquadUUIDs: request.SquadUUIDs,
		})
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, adminEntitlementResponse{Purchase: purchase, UserID: purchase.UserID})
}

func (s *Server) adminRefundEntitlement(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		Reason         string `json:"reason"`
		AmountTXBMinor string `json:"amountTxbMinor"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ENTITLEMENT_REFUND", "A refund reason is required.")
		return
	}
	amount, err := strconv.ParseInt(request.AmountTXBMinor, 10, 64)
	if err != nil || amount <= 0 {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_ENTITLEMENT_REFUND_AMOUNT", "Refund amount must be a positive TXB minor amount.")
		return
	}
	receipt, err := s.deps.AdminUsers.RefundEntitlement(r.Context(), currentUser(r).ID,
		chiURLParam(r, "userId"), chiURLParam(r, "entitlementId"), key, request.Reason, amount)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) adminReplaceCombo(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		ComboID         string   `json:"comboId"`
		AddonSquadUUIDs []string `json:"addonSquadUuids"`
		Reason          string   `json:"reason"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_COMBO_REPLACEMENT", "Combo, add-ons, and reason are required.")
		return
	}
	receipt, err := s.deps.AdminUsers.ReplaceCombo(r.Context(), currentUser(r).ID, chiURLParam(r, "userId"), key,
		admin.ComboReplacement{ComboID: request.ComboID, AddonSquadUUIDs: request.AddonSquadUUIDs, Reason: request.Reason})
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

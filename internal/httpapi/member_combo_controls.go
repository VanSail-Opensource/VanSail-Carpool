package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func (s *Server) memberComboControls(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	controls, err := s.deps.PurchaseOperations.ComboControls(r.Context(), user.ID)
	if err != nil {
		s.writeComboControlError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, controls)
}

func (s *Server) memberSwitchSquad(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		PurchaseID string `json:"purchaseId"`
		Enabled    *bool  `json:"enabled"`
	}
	if err := decodeJSON(w, r, &body); err != nil || body.Enabled == nil || strings.TrimSpace(body.PurchaseID) == "" {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_SQUAD_SWITCH", "Provide the current purchase and enabled state.")
		return
	}
	receipt, err := s.deps.PurchaseOperations.SwitchSquad(r.Context(), user.ID, body.PurchaseID, chiURLParam(r, "uuid"), *body.Enabled, key)
	if err != nil {
		s.writeComboControlError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) memberEarlyActivationQuote(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	quote, err := s.deps.PurchaseOperations.EarlyActivation(r.Context(), user.ID, chiURLParam(r, "id"))
	if err != nil {
		s.writeComboControlError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

func (s *Server) memberActivateEarly(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		CurrentID    string `json:"currentPurchaseId"`
		Confirmation string `json:"confirmation"`
	}
	if err := decodeJSON(w, r, &body); err != nil || strings.TrimSpace(body.CurrentID) == "" {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_EARLY_ACTIVATION", "Provide the current purchase and confirmation.")
		return
	}
	receipt, err := s.deps.PurchaseOperations.ActivateEarly(r.Context(), user.ID, body.CurrentID, chiURLParam(r, "id"), body.Confirmation, key)
	if err != nil {
		s.writeComboControlError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) writeComboControlError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, purchaseops.ErrLastEnabledSquad) {
		s.writeError(w, r, http.StatusConflict, "LAST_SQUAD_REQUIRED", "Keep at least one internal squad enabled.")
		return
	}
	if errors.Is(err, purchaseops.ErrConfirmationRequired) {
		s.writeError(w, r, http.StatusBadRequest, "CONFIRMATION_REQUIRED", "Type the early activation confirmation text.")
		return
	}
	s.writeMemberOperationError(w, r, err, true)
}

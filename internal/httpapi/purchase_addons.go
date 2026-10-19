package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/go-chi/chi/v5"
)

type purchaseAddonRequest struct {
	AddonSquadProductIDs []string          `json:"addonSquadProductIds"`
	SquadActivationCodes map[string]string `json:"squadActivationCodes"`
}

func (s *Server) purchaseAddonQuote(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	var request purchaseAddonRequest
	if err := decodeJSON(w, r, &request); err != nil || len(request.AddonSquadProductIDs) == 0 {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Select at least one optional squad.")
		return
	}
	quote, err := s.deps.Catalog.QuoteAddons(r.Context(), user, chi.URLParam(r, "id"), request.AddonSquadProductIDs)
	if err != nil {
		s.writePurchaseAddonError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

func (s *Server) purchaseAddons(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	var request purchaseAddonRequest
	if err := decodeJSON(w, r, &request); err != nil || len(request.AddonSquadProductIDs) == 0 {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Select at least one optional squad.")
		return
	}
	idempotencyKey, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	purchase, err := s.deps.Catalog.AddAddons(r.Context(), user, chi.URLParam(r, "id"), request.AddonSquadProductIDs, request.SquadActivationCodes, idempotencyKey)
	if err != nil {
		s.writePurchaseAddonError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, purchase)
}

func (s *Server) writePurchaseAddonError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrConflict):
		s.writeError(w, r, http.StatusConflict, "OPERATION_CONFLICT", "Another entitlement change is pending or this request key was already used.")
	case errors.Is(err, database.ErrNotFound):
		s.writeError(w, r, http.StatusNotFound, "PURCHASE_OR_SQUAD_NOT_FOUND", "The active ride or selected squad is unavailable.")
	case errors.Is(err, database.ErrPurchaseNotActive):
		s.writeError(w, r, http.StatusConflict, "PURCHASE_NOT_ACTIVE", "Squads can only be added to the active ride.")
	case errors.Is(err, database.ErrQueuedPurchase):
		s.writeError(w, r, http.StatusConflict, "SQUAD_ADDITION_QUEUED", "A queued ride already controls the next renewal.")
	case errors.Is(err, database.ErrSquadAlreadyAdded):
		s.writeError(w, r, http.StatusConflict, "SQUAD_ALREADY_ADDED", "A selected squad is already part of this ride.")
	case errors.Is(err, database.ErrStockUnavailable):
		s.writeError(w, r, http.StatusConflict, "SQUAD_STOCK_UNAVAILABLE", "A selected squad is currently full.")
	case errors.Is(err, database.ErrInsufficientBalance):
		s.writeError(w, r, http.StatusConflict, "INSUFFICIENT_BALANCE", "Your TXB balance is too low for this squad addition.")
	case errors.Is(err, database.ErrActivationCodeRequired):
		s.writeError(w, r, http.StatusUnprocessableEntity, "SQUAD_ACTIVATION_REQUIRED", "An activation code is required for a selected squad.")
	case errors.Is(err, database.ErrActivationCodeInvalid):
		s.writeError(w, r, http.StatusUnprocessableEntity, "SQUAD_ACTIVATION_INVALID", "A selected squad activation code is invalid.")
	case errors.Is(err, database.ErrActivationCodeExtra):
		s.writeError(w, r, http.StatusUnprocessableEntity, "SQUAD_ACTIVATION_EXTRA", "Activation codes may only be sent for selected squads.")
	default:
		s.deps.Logger.Error("add purchase squads", "request_id", middlewareRequestID(r), "user_id", currentUser(r).ID, "purchase_id", chi.URLParam(r, "id"), "error", err)
		s.writeError(w, r, http.StatusConflict, "SQUAD_ADDITION_FAILED", "The squad addition could not be completed.")
	}
}

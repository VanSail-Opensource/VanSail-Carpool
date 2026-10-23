package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"net/http"
	"time"
)

func (s *Server) continueOnboarding(w http.ResponseWriter, r *http.Request) {
	user, err := s.deps.Accounts.BeginAgreement(r.Context(), currentUser(r))
	if err != nil {
		s.writeError(w, r, http.StatusConflict, "ONBOARDING_STATE_CONFLICT", "Onboarding could not be advanced.")
		return
	}
	writeJSON(w, http.StatusOK, authState{Authenticated: true, User: mapUser(user)})
}

func (s *Server) acceptAgreement(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Revision     int      `json:"revision"`
		AgreementIDs []string `json:"agreementIds"`
	}
	if err := decodeJSON(w, r, &request); err != nil || request.Revision <= 0 || len(request.AgreementIDs) == 0 {
		s.writeError(w, r, http.StatusBadRequest, "AGREEMENT_REQUIRED", "Select every current agreement to continue.")
		return
	}
	current := currentUser(r)
	user, err := s.deps.Accounts.AcceptAgreementRevision(r.Context(), current, request.Revision, request.AgreementIDs)
	if err != nil {
		status, code := http.StatusBadGateway, "XNODE_UNAVAILABLE"
		switch {
		case errors.Is(err, accounts.ErrAgreementRevisionConflict):
			status, code = http.StatusConflict, "AGREEMENT_OUTDATED"
		case errors.Is(err, accounts.ErrProxyIdentityConflict):
			status, code = http.StatusConflict, "XNODE_IDENTITY_CONFLICT"
		case errors.Is(err, accounts.ErrAgreementStateConflict):
			status, code = http.StatusConflict, "ONBOARDING_STATE_CONFLICT"
		case errors.Is(err, accounts.ErrUsernameUnavailable) || errors.Is(err, database.ErrConflict):
			status, code = http.StatusConflict, "ONBOARDING_CONFLICT"
		}
		s.deps.Logger.Warn("onboarding agreement rejected", "request_id", middlewareRequestID(r), "user_id", current.ID, "code", code, "error", err)
		s.writeError(w, r, status, code, "The account could not be completed yet. You can safely retry.")
		return
	}
	writeJSON(w, http.StatusOK, authState{Authenticated: true, User: mapUser(user)})
}

type dashboardResponse struct {
	User               userResponse              `json:"user"`
	Balance            model.Money               `json:"balance"`
	ActivePurchase     *model.Purchase           `json:"activePurchase"`
	QueuedPurchase     *model.Purchase           `json:"queuedPurchase"`
	AutoRenewalFailure *model.AutoRenewalFailure `json:"autoRenewalFailure"`
	Statistics         *model.Statistics         `json:"statistics"`
	SubscriptionURL    *string                   `json:"subscriptionUrl"`
	StatisticsStale    bool                      `json:"statisticsStale"`
	StatisticsWarning  string                    `json:"statisticsWarning"`
	FetchedAt          time.Time                 `json:"fetchedAt"`
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	dashboard, err := s.deps.Catalog.Dashboard(r.Context(), user)
	if err != nil {
		s.writeError(w, r, http.StatusBadGateway, "DASHBOARD_UNAVAILABLE", "Live account data is temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, dashboardResponse{User: mapUser(dashboard.User), Balance: dashboard.Balance, ActivePurchase: dashboard.ActivePurchase,
		QueuedPurchase: dashboard.QueuedPurchase, AutoRenewalFailure: dashboard.AutoRenewalFailure, Statistics: dashboard.Statistics, SubscriptionURL: dashboard.SubscriptionURL,
		StatisticsStale: dashboard.StatisticsStale, StatisticsWarning: dashboard.StatisticsWarning, FetchedAt: dashboard.FetchedAt})
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	snapshot, err := s.deps.Catalog.CatalogForUser(r.Context(), user)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "CATALOG_UNAVAILABLE", "The catalog could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) purchase(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	var request struct {
		ComboID              string            `json:"comboId"`
		AddonSquadProductIDs []string          `json:"addonSquadProductIds"`
		CouponGrantID        string            `json:"couponGrantId"`
		SquadActivationCodes map[string]string `json:"squadActivationCodes"`
	}
	if err := decodeJSON(w, r, &request); err != nil || request.ComboID == "" {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Select a combo to continue.")
		return
	}
	idempotencyKey, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	purchase, err := s.deps.Catalog.PurchaseWithCoupon(r.Context(), user, request.ComboID, request.AddonSquadProductIDs, request.CouponGrantID, request.SquadActivationCodes, idempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, catalog.ErrAutoRenewalEnabled):
			s.writeError(w, r, http.StatusConflict, "AUTO_RENEW_ENABLED", "Turn off automatic renewal before selecting a new combo.")
		case errors.Is(err, catalog.ErrNoAccessibleNodes):
			s.writeError(w, r, http.StatusConflict, "NO_ACCESSIBLE_NODES", "The selected catalog item has no accessible nodes.")
		case errors.Is(err, database.ErrStockUnavailable):
			s.writeError(w, r, http.StatusConflict, "SQUAD_STOCK_UNAVAILABLE", "A selected squad is currently full.")
		case errors.Is(err, database.ErrInsufficientBalance):
			s.writeError(w, r, http.StatusConflict, "INSUFFICIENT_BALANCE", "Your TXB balance is too low for this purchase.")
		case errors.Is(err, database.ErrActivationCodeRequired):
			s.writeError(w, r, http.StatusUnprocessableEntity, "SQUAD_ACTIVATION_REQUIRED", "An activation code is required for a selected squad.")
		case errors.Is(err, database.ErrActivationCodeInvalid):
			s.writeError(w, r, http.StatusUnprocessableEntity, "SQUAD_ACTIVATION_INVALID", "A selected squad activation code is invalid.")
		case errors.Is(err, database.ErrActivationCodeExtra):
			s.writeError(w, r, http.StatusUnprocessableEntity, "SQUAD_ACTIVATION_EXTRA", "Activation codes may only be sent for selected squads.")
		case errors.Is(err, database.ErrNotFound):
			s.writeError(w, r, http.StatusNotFound, "CATALOG_ITEM_NOT_FOUND", "The selected catalog item is unavailable.")
		default:
			s.writeError(w, r, http.StatusConflict, "PURCHASE_FAILED", "The purchase could not be completed.")
		}
		return
	}
	writeJSON(w, http.StatusCreated, purchase)
}

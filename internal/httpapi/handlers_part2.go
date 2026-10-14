package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) purchaseQuote(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	var request struct {
		ComboID              string   `json:"comboId"`
		AddonSquadProductIDs []string `json:"addonSquadProductIds"`
		CouponGrantID        string   `json:"couponGrantId"`
	}
	if err := decodeJSON(w, r, &request); err != nil || request.ComboID == "" {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Select a combo to continue.")
		return
	}
	quote, err := s.deps.Catalog.Quote(r.Context(), user, request.ComboID, request.AddonSquadProductIDs, request.CouponGrantID)
	if err != nil {
		status := http.StatusConflict
		code := "QUOTE_FAILED"
		if errors.Is(err, catalog.ErrAutoRenewalEnabled) {
			code = "AUTO_RENEW_ENABLED"
		}
		if errors.Is(err, catalog.ErrNoAccessibleNodes) {
			code = "NO_ACCESSIBLE_NODES"
		}
		if errors.Is(err, database.ErrStockUnavailable) {
			code = "SQUAD_STOCK_UNAVAILABLE"
		}
		if errors.Is(err, database.ErrNotFound) {
			status = http.StatusNotFound
			code = "CATALOG_ITEM_NOT_FOUND"
		}
		s.writeError(w, r, status, code, "The selected price or entitlement is no longer available.")
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

func (s *Server) purchases(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Catalog.Purchases(r.Context(), currentUser(r).ID)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "PURCHASES_UNAVAILABLE", "Purchase history could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) revokeSubscription(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	receipt, err := s.deps.Catalog.QueueSubscriptionRevoke(r.Context(), currentUser(r), key)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, database.ErrConflict) {
			status = http.StatusConflict
		}
		s.writeError(w, r, status, "REVOKE_FAILED", "The subscription rotation could not be queued.")
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) balance(w http.ResponseWriter, r *http.Request) {
	balance, err := s.deps.Store.Balance(r.Context(), currentUser(r).ID)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "BALANCE_UNAVAILABLE", "Balance could not be loaded.")
		return
	}
	methods, err := s.deps.Billing.Methods(r.Context())
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "PAYMENT_METHODS_UNAVAILABLE", "Payment methods could not be loaded.")
		return
	}
	limits, err := s.deps.Store.AddTXBBounds(r.Context())
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "BILLING_LIMITS_UNAVAILABLE", "Payment amount limits could not be loaded.")
		return
	}
	var pending *model.PaymentOrder
	order, pendingErr := s.deps.Store.LatestValidUnpaidPaymentOrder(r.Context(), currentUser(r).ID, time.Now().UTC())
	if pendingErr == nil {
		pending = &order
	} else if !errors.Is(pendingErr, database.ErrNotFound) {
		s.writeError(w, r, http.StatusInternalServerError, "PAYMENT_ORDER_UNAVAILABLE", "Pending payment could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"balance": balance, "paymentMethods": methods, "addAmountLimits": limits, "pendingPaymentOrder": pending,
	})
}

func (s *Server) ledger(w http.ResponseWriter, r *http.Request) {
	size := 25
	if raw := r.URL.Query().Get("size"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > 100 {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_PAGE_SIZE", "Page size must be between 1 and 100.")
			return
		}
		size = parsed
	}
	items, nextCursor, err := s.deps.Store.ListLedgerPage(r.Context(), currentUser(r).ID, r.URL.Query().Get("cursor"), size)
	if err != nil {
		if errors.Is(err, database.ErrInvalidCursor) {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "The pagination cursor is invalid.")
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, "LEDGER_UNAVAILABLE", "Ledger history could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page": map[string]any{"nextCursor": nextCursor}})
}

func (s *Server) requireOnboarded(w http.ResponseWriter, r *http.Request, user model.User) bool {
	if user.OnboardingState == "complete" {
		return true
	}
	s.writeError(w, r, http.StatusConflict, "ONBOARDING_REQUIRED", "Complete onboarding to use this feature.")
	return false
}

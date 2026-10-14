package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) createPaymentOrder(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	idempotencyKey, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		MethodID string `json:"methodId"`
		TXBMinor string `json:"txbMinor"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Provider and TXB amount are required.")
		return
	}
	amount, err := strconv.ParseInt(request.TXBMinor, 10, 64)
	if err != nil || !billing.CanonicalMethodID(request.MethodID) {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_AMOUNT", "TXB amount must be integer hundredths.")
		return
	}
	operation, err := s.deps.Billing.QueueOrder(r.Context(), user, strings.ToLower(request.MethodID), amount, idempotencyKey)
	if err != nil {
		if s.deps.Logger != nil {
			s.deps.Logger.Warn("payment provider order creation failed", "request_id", middlewareRequestID(r), "method_id", strings.ToLower(request.MethodID), "amount_minor", amount, "error", err)
		}
		if errors.Is(err, billing.ErrInvalidOrder) {
			s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_PAYMENT_ORDER", "The provider or TXB amount is invalid.")
		} else if errors.Is(err, billing.ErrProviderDisabled) {
			s.writeError(w, r, http.StatusConflict, "PROVIDER_DISABLED", "This payment provider is not available.")
		} else if errors.Is(err, database.ErrConflict) {
			s.writeError(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "The idempotency key is already used for another payment request.")
		} else {
			s.writeError(w, r, http.StatusInternalServerError, "PAYMENT_CREATE_FAILED", "The payment operation could not be created.")
		}
		return
	}
	writeJSON(w, http.StatusAccepted, operation)
}

func (s *Server) cancelPaymentOrder(w http.ResponseWriter, r *http.Request) {
	idempotencyKey, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	operation, err := s.deps.Billing.QueueCancellation(r.Context(), chiURLParam(r, "id"), currentUser(r).ID, idempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			s.writeError(w, r, http.StatusNotFound, "PAYMENT_NOT_FOUND", "Payment order not found.")
		case errors.Is(err, database.ErrConflict):
			s.writeError(w, r, http.StatusConflict, "PAYMENT_OPERATION_CONFLICT", "The payment cannot be cancelled or the idempotency key conflicts with another request.")
		default:
			s.writeError(w, r, http.StatusInternalServerError, "PAYMENT_CANCEL_FAILED", "The payment could not be cancelled.")
		}
		return
	}
	writeJSON(w, http.StatusAccepted, operation)
}

func (s *Server) paymentOrder(w http.ResponseWriter, r *http.Request) {
	order, err := s.deps.Billing.OrderForUser(r.Context(), chiURLParam(r, "id"), currentUser(r).ID)
	if err != nil {
		s.writeError(w, r, http.StatusNotFound, "PAYMENT_NOT_FOUND", "Payment order not found.")
		return
	}
	writeJSON(w, http.StatusOK, order)
}

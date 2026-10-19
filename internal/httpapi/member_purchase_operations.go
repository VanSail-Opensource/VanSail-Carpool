package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Server) memberRefundQuote(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	quote, err := s.deps.PurchaseOperations.RefundQuote(r.Context(), user.ID, chiURLParam(r, "id"))
	if err != nil {
		s.writeMemberOperationError(w, r, err, false)
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

func (s *Server) memberRefund(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	receipt, err := s.deps.PurchaseOperations.RefundPurchase(r.Context(), user.ID, chiURLParam(r, "id"), key)
	if err != nil {
		s.writeMemberOperationError(w, r, err, true)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) memberOperation(w http.ResponseWriter, r *http.Request) {
	receipt, err := s.deps.PurchaseOperations.Operation(r.Context(), currentUser(r).ID, chiURLParam(r, "id"))
	if err != nil {
		s.writeMemberOperationError(w, r, err, false)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}

func (s *Server) writeMemberOperationError(w http.ResponseWriter, r *http.Request, err error, mutation bool) {
	switch {
	case errors.Is(err, purchaseops.ErrNotFound), errors.Is(err, database.ErrNotFound):
		s.writeError(w, r, http.StatusNotFound, "PURCHASE_NOT_FOUND", "The purchase could not be found.")
	case errors.Is(err, purchaseops.ErrIneligible):
		s.writeError(w, r, http.StatusUnprocessableEntity, "PURCHASE_OPERATION_INELIGIBLE", "The purchase is no longer eligible for this operation.")
	case errors.Is(err, database.ErrInsufficientBalance):
		s.writeError(w, r, http.StatusConflict, "INSUFFICIENT_BALANCE", "Your TXB balance is too low for this operation.")
	case errors.Is(err, database.ErrConflict), errors.Is(err, xnode.ErrConflict), errors.Is(err, purchaseops.ErrIdempotencyConflict):
		s.writeError(w, r, http.StatusConflict, "OPERATION_CONFLICT", "The operation conflicts with current state or an existing idempotency key.")
	default:
		if mutation {
			s.writeError(w, r, http.StatusInternalServerError, "PURCHASE_OPERATION_FAILED", "The operation could not be created.")
		} else {
			s.writeError(w, r, http.StatusBadGateway, "PURCHASE_QUOTE_UNAVAILABLE", "The operation quote is temporarily unavailable.")
		}
	}
}

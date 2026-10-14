package httpapi

import (
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Server) adminPayments(w http.ResponseWriter, r *http.Request) {
	query, ok := s.parseAdminInventoryQuery(w, r, paymentPageStatuses)
	if !ok {
		return
	}
	items, nextCursor, err := s.deps.Store.ListAdminPaymentOrdersPage(r.Context(), query.Cursor, query.Search, query.Status, query.Limit)
	if err != nil {
		s.writeAdminPageFailure(w, r, err)
		return
	}
	type adminPayment struct {
		model.PaymentOrder
		UserID string `json:"userId"`
	}
	response := make([]adminPayment, 0, len(items))
	for _, item := range items {
		response = append(response, adminPayment{PaymentOrder: item, UserID: item.UserID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": response, "page": map[string]any{"nextCursor": nextCursor}})
}

func (s *Server) adminRefundPayment(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_REFUND", "A refund reason is required.")
		return
	}
	receipt, err := s.deps.Admin.QueueRefund(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), request.Reason, key)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) adminCourtesyCreditPayment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_COURTESY_CREDIT", "A courtesy-credit reason is required.")
		return
	}
	credit, err := s.deps.Admin.CourtesyCredit(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), request.Reason)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, credit)
}

func (s *Server) adminRefunds(w http.ResponseWriter, r *http.Request) {
	query, ok := s.parseAdminInventoryQuery(w, r, refundPageStatuses)
	if !ok {
		return
	}
	items, nextCursor, err := s.deps.Store.ListAdminRefundsPage(r.Context(), query.Cursor, query.Search, query.Status, query.Limit)
	if err != nil {
		s.writeAdminPageFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page": map[string]any{"nextCursor": nextCursor}})
}

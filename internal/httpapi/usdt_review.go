package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func (s *Server) adminUSDTReview(w http.ResponseWriter, r *http.Request) {
	if !s.usdtAvailable(w, r) {
		return
	}
	cursor := r.URL.Query().Get("cursor")
	if len(cursor) > 64 {
		s.usdtError(w, r, usdt.ErrConfiguration)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if orderID := r.URL.Query().Get("orderId"); orderID != "" {
		if len(orderID) > 128 {
			s.usdtError(w, r, usdt.ErrConfiguration)
			return
		}
		result, err := s.deps.USDT.MatchingEvidence(r.Context(), orderID)
		if err != nil {
			s.usdtError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	result, err := s.deps.USDT.Review(r.Context(), cursor, limit)
	if err != nil {
		s.usdtError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) adminUSDTResolve(w http.ResponseWriter, r *http.Request) {
	if !s.usdtAvailable(w, r) {
		return
	}
	var input struct {
		OrderID string `json:"orderId"`
		Reason  string `json:"reason"`
		Action  string `json:"action"`
	}
	if err := decodeJSON(w, r, &input); err != nil || (input.Action != "dismiss" && len(input.OrderID) == 0) || len(input.OrderID) > 128 || len(input.Reason) == 0 || len(input.Reason) > 500 || (input.Action != "" && input.Action != "settle" && input.Action != "dismiss") {
		s.usdtError(w, r, usdt.ErrConfiguration)
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 64 {
		s.usdtError(w, r, usdt.ErrConfiguration)
		return
	}
	if input.Action == "dismiss" {
		changed, err := s.deps.USDT.Dismiss(r.Context(), id, currentUser(r).ID, input.Reason)
		if err != nil {
			s.usdtError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"orderId": "", "transferId": id, "credited": false, "replayed": !changed})
		return
	}
	order, changed, err := s.deps.USDT.Resolve(r.Context(), id, input.OrderID, currentUser(r).ID, input.Reason)
	if err != nil {
		s.usdtError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		OrderID    string `json:"orderId"`
		TransferID string `json:"transferId"`
		Credited   bool   `json:"credited"`
		Replayed   bool   `json:"replayed"`
	}{order.ID, id, true, !changed})
}

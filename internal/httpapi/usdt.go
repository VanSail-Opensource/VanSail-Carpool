package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/database"
	"github.com/txyyddss/Remna-User-Panel/internal/usdt"
)

func (s *Server) mountUSDT(r chi.Router) {
	r.With(s.requireAdmin).Get("/api/v1/admin/usdt", s.adminUSDTConfig)
	r.With(s.requireAdmin).Put("/api/v1/admin/usdt", s.adminSaveUSDTConfig)
	r.With(s.requireAdmin).Get("/api/v1/admin/usdt/status", s.adminUSDTStatus)
	r.With(s.requireAdmin).Post("/api/v1/admin/usdt/check", s.adminUSDTCheck)
	r.With(s.requireAdmin).Get("/api/v1/admin/usdt/review", s.adminUSDTReview)
	r.With(s.requireAdmin).Put("/api/v1/admin/usdt/review/{id}/resolution", s.adminUSDTResolve)
}
func (s *Server) usdtAvailable(w http.ResponseWriter, r *http.Request) bool {
	if s.deps.USDT == nil {
		s.usdtError(w, r, usdt.ErrUnavailable)
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	return true
}
func (s *Server) adminUSDTConfig(w http.ResponseWriter, r *http.Request) {
	if !s.usdtAvailable(w, r) {
		return
	}
	result, err := s.deps.USDT.Config(r.Context())
	if err != nil {
		s.usdtError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) adminSaveUSDTConfig(w http.ResponseWriter, r *http.Request) {
	if !s.usdtAvailable(w, r) {
		return
	}
	var input usdt.Config
	if err := decodeJSON(w, r, &input); err != nil {
		s.usdtError(w, r, usdt.ErrConfiguration)
		return
	}
	result, err := s.deps.USDT.SaveConfig(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.usdtError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) adminUSDTStatus(w http.ResponseWriter, r *http.Request) {
	if !s.usdtAvailable(w, r) {
		return
	}
	result, err := s.deps.USDT.Status(r.Context())
	if err != nil {
		s.usdtError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) adminUSDTCheck(w http.ResponseWriter, r *http.Request) {
	if !s.usdtAvailable(w, r) {
		return
	}
	var input struct {
		Network string `json:"network"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.usdtError(w, r, usdt.ErrConfiguration)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	result, err := s.deps.USDT.Check(ctx, input.Network)
	if err != nil {
		s.usdtError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
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
	}
	if err := decodeJSON(w, r, &input); err != nil || len(input.OrderID) == 0 || len(input.OrderID) > 128 || len(input.Reason) == 0 || len(input.Reason) > 500 {
		s.usdtError(w, r, usdt.ErrConfiguration)
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 64 {
		s.usdtError(w, r, usdt.ErrConfiguration)
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
func (s *Server) usdtError(w http.ResponseWriter, r *http.Request, err error) {
	code, status := usdt.ErrorCode(err), http.StatusBadGateway
	if errors.Is(err, usdt.ErrConfiguration) {
		status = http.StatusUnprocessableEntity
	}
	if errors.Is(err, usdt.ErrConflict) || errors.Is(err, database.ErrConflict) {
		code, status = "USDT_TRANSFER_CONFLICT", http.StatusConflict
	}
	if errors.Is(err, database.ErrNotFound) || errors.Is(err, usdt.ErrQuoteNotFound) {
		code, status = "USDT_NOT_FOUND", http.StatusNotFound
	}
	if errors.Is(err, usdt.ErrUnavailable) {
		status = http.StatusServiceUnavailable
	}
	s.writeError(w, r, status, code, "USDT payment operation could not complete.")
}

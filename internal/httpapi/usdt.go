package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
	"github.com/go-chi/chi/v5"
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
		Rail    string `json:"rail"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.usdtError(w, r, usdt.ErrConfiguration)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	key := input.Network
	if input.Rail != "" {
		spec, ok := usdt.Network(input.Rail)
		if !ok || spec.Network != input.Network {
			s.usdtError(w, r, usdt.ErrConfiguration)
			return
		}
		key = spec.Rail
	}
	result, err := s.deps.USDT.Check(ctx, key)
	if err != nil {
		s.usdtError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
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

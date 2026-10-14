package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/go-chi/chi/v5"
)

type automaticRenewalRequest struct {
	Enabled *bool `json:"enabled"`
}

func (s *Server) automaticRenewal(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	status, err := s.deps.Catalog.AutomaticRenewal(r.Context(), user, chi.URLParam(r, "id"))
	if err != nil {
		s.writeAutomaticRenewalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) updateAutomaticRenewal(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	var request automaticRenewalRequest
	if err := decodeJSON(w, r, &request); err != nil || request.Enabled == nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_AUTO_RENEWAL", "Provide the automatic renewal state.")
		return
	}
	status, err := s.deps.Catalog.SetAutomaticRenewal(r.Context(), user, chi.URLParam(r, "id"), *request.Enabled)
	if err != nil {
		if errors.Is(err, catalog.ErrAutoRenewalIneligible) {
			s.writeError(w, r, http.StatusConflict, "AUTO_RENEWAL_INELIGIBLE", "Automatic renewal cannot be enabled for this term.")
			return
		}
		s.writeAutomaticRenewalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) writeAutomaticRenewalError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrNotFound):
		s.writeError(w, r, http.StatusNotFound, "PURCHASE_NOT_FOUND", "The current ride could not be found.")
	case errors.Is(err, database.ErrConflict):
		s.writeError(w, r, http.StatusConflict, "AUTO_RENEWAL_CONFLICT", "Automatic renewal is no longer available for this term.")
	default:
		user := currentUser(r)
		s.deps.Logger.Warn("load automatic renewal state", "request_id", middlewareRequestID(r), "method", r.Method, "user_id", user.ID, "purchase_id", chi.URLParam(r, "id"), "error", err)
		s.writeError(w, r, http.StatusBadGateway, "AUTO_RENEWAL_UNAVAILABLE", "Automatic renewal is temporarily unavailable.")
	}
}

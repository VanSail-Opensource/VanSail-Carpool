package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/database"
)

func (s *Server) mountIPLookup(r chi.Router) {
	r.Get("/api/v1/ip-lookup", s.ipLookupState)
	r.Get("/api/v1/ip-lookup/details", s.ipLookupDetails)
	r.Post("/api/v1/ip-lookup/quote", s.ipLookupQuote)
	r.Post("/api/v1/ip-lookup/checks", s.ipLookupSubmit)
	r.Get("/api/v1/ip-lookup/checks/{id}", s.ipLookupCheck)
	r.With(s.requireAdmin).Get("/api/v1/admin/ip-lookup", s.adminIPLookupSettings)
	r.With(s.requireAdmin).Put("/api/v1/admin/ip-lookup", s.adminSaveIPLookupSettings)
}

func (s *Server) ipLookupState(w http.ResponseWriter, r *http.Request) {
	if s.deps.IPLookup == nil {
		writeJSON(w, http.StatusOK, iplookup.State{})
		return
	}
	result, err := s.deps.IPLookup.State(r.Context(), currentUser(r).ID)
	if err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) ipLookupQuote(w http.ResponseWriter, r *http.Request) {
	if !s.ipLookupMember(w, r) {
		return
	}
	var input struct {
		IP      string `json:"ip"`
		Refresh bool   `json:"refresh"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.ipLookupError(w, r, &iplookup.CodeError{Code: "IP_LOOKUP_INVALID_IP"})
		return
	}
	result, err := s.deps.IPLookup.PreviewQuote(r.Context(), currentUser(r).ID, input.IP, input.Refresh)
	if err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) ipLookupSubmit(w http.ResponseWriter, r *http.Request) {
	if !s.ipLookupMember(w, r) {
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input iplookup.Quote
	if err := decodeJSON(w, r, &input); err != nil {
		s.ipLookupError(w, r, &iplookup.CodeError{Code: "IP_LOOKUP_QUOTE_CHANGED"})
		return
	}
	result, err := s.deps.IPLookup.Submit(r.Context(), currentUser(r).ID, key, input)
	if err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *Server) ipLookupCheck(w http.ResponseWriter, r *http.Request) {
	if !s.ipLookupMember(w, r) {
		return
	}
	result, err := s.deps.IPLookup.Check(r.Context(), currentUser(r).ID, chiURLParam(r, "id"))
	if err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) ipLookupMember(w http.ResponseWriter, r *http.Request) bool {
	if !s.requireOnboarded(w, r, currentUser(r)) {
		return false
	}
	if s.deps.IPLookup == nil {
		s.ipLookupError(w, r, &iplookup.CodeError{Code: "IP_LOOKUP_DISABLED"})
		return false
	}
	return true
}

func (s *Server) adminIPLookupSettings(w http.ResponseWriter, r *http.Request) {
	result, err := s.deps.Settings.IPLookupSettings(r.Context())
	if err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) adminSaveIPLookupSettings(w http.ResponseWriter, r *http.Request) {
	var input iplookup.AdminSettings
	if err := decodeJSON(w, r, &input); err != nil {
		s.ipLookupError(w, r, &iplookup.CodeError{Code: "IP_LOOKUP_INVALID_CONFIG"})
		return
	}
	if err := s.deps.Settings.PutIPLookupSettings(r.Context(), currentUser(r).ID, input); err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	s.adminIPLookupSettings(w, r)
}

func (s *Server) ipLookupError(w http.ResponseWriter, r *http.Request, err error) {
	code, status := iplookup.ErrorCode(err), http.StatusBadGateway
	var typed *iplookup.CodeError
	if errors.As(err, &typed) {
		status = http.StatusUnprocessableEntity
	}
	if code == "IP_LOOKUP_QUOTE_CHANGED" || code == "IP_LOOKUP_BUSY" || code == "IP_LOOKUP_REFRESH_REQUIRED" {
		status = http.StatusConflict
	}
	if code == "IP_DETAILS_BUSY" {
		status = http.StatusConflict
	}
	if code == "IP_DETAILS_RATE_LIMITED" {
		status = http.StatusTooManyRequests
		w.Header().Set("Retry-After", "10")
	}
	if errors.Is(err, database.ErrInsufficientBalance) {
		code, status = "INSUFFICIENT_BALANCE", http.StatusConflict
	}
	if errors.Is(err, database.ErrConflict) {
		code, status = "IP_LOOKUP_CONFLICT", http.StatusConflict
	}
	if errors.Is(err, database.ErrNotFound) {
		code, status = "IP_LOOKUP_NOT_FOUND", http.StatusNotFound
	}
	s.writeError(w, r, status, code, "IP Lookup could not complete the request.")
}

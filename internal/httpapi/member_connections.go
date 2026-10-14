package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) requestConnections(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	scan, err := s.deps.Connections.Request(r.Context(), user, key)
	if err != nil {
		s.writeConnectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, scan)
}

func (s *Server) pollConnections(w http.ResponseWriter, r *http.Request) {
	scan, err := s.deps.Connections.Poll(r.Context(), currentUser(r).ID, chiURLParam(r, "id"))
	if err != nil {
		if errors.Is(err, connections.ErrScanNotFound) || errors.Is(err, database.ErrNotFound) {
			s.writeError(w, r, http.StatusNotFound, "CONNECTION_SCAN_NOT_FOUND", "The connection scan could not be found.")
			return
		}
		s.writeError(w, r, http.StatusBadGateway, "CONNECTION_SCAN_UNAVAILABLE", "Connection progress is temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, scan)
}

func (s *Server) dropConnection(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		Handle string `json:"handle"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_CONNECTION_HANDLE", "A valid connection handle is required.")
		return
	}
	receipt, err := s.deps.ConnectionDrops.Drop(r.Context(), user.ID, request.Handle, key)
	if err != nil {
		s.writeConnectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) writeConnectionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, connections.ErrInvalidHandle):
		s.writeError(w, r, http.StatusBadRequest, "INVALID_CONNECTION_HANDLE", "The connection handle is invalid or expired.")
	case errors.Is(err, connections.ErrScanNotFound), errors.Is(err, database.ErrNotFound):
		s.writeError(w, r, http.StatusNotFound, "CONNECTION_SCAN_NOT_FOUND", "The connection scan could not be found.")
	case errors.Is(err, connections.ErrIdentityRequired):
		s.writeError(w, r, http.StatusConflict, "REMNAWAVE_IDENTITY_REQUIRED", "Complete account synchronization before scanning connections.")
	case errors.Is(err, database.ErrConflict):
		s.writeError(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "The idempotency key is already used for another request.")
	default:
		s.writeError(w, r, http.StatusInternalServerError, "CONNECTION_OPERATION_FAILED", "The connection operation could not be created.")
	}
}

package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Server) writeConnectionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, connections.ErrScanForbidden):
		s.writeError(w, r, http.StatusForbidden, "ADMIN_REQUIRED", "Administrator access is required.")
	case errors.Is(err, connections.ErrScanUnavailable):
		s.writeError(w, r, http.StatusServiceUnavailable, "CONNECTION_SCAN_UNAVAILABLE", "The connection scan is temporarily unavailable. Retry the read shortly.")
	case errors.Is(err, xnode.ErrUnavailable), errors.Is(err, xnode.ErrNotFound), errors.Is(err, xnode.ErrInvalid):
		s.xnodeError(w, r, err)
	case errors.Is(err, connections.ErrInvalidHandle):
		s.writeError(w, r, http.StatusBadRequest, "INVALID_CONNECTION_HANDLE", "The connection handle is invalid or expired.")
	case errors.Is(err, connections.ErrScanNotFound), errors.Is(err, database.ErrNotFound):
		s.writeError(w, r, http.StatusNotFound, "CONNECTION_SCAN_NOT_FOUND", "The connection scan could not be found.")
	case errors.Is(err, connections.ErrIdentityRequired):
		s.writeError(w, r, http.StatusConflict, "XNODE_IDENTITY_REQUIRED", "Complete account synchronization before scanning connections.")
	case errors.Is(err, database.ErrConflict):
		s.writeError(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "The idempotency key is already used for another request.")
	default:
		s.writeError(w, r, http.StatusInternalServerError, "CONNECTION_OPERATION_FAILED", "The connection operation could not be created.")
	}
}

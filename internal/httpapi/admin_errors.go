package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
)

func (s *Server) adminFailure(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusUnprocessableEntity, "ADMIN_OPERATION_FAILED", err.Error()
	switch {
	case errors.Is(err, compensation.ErrInvalid):
		status, code, message = http.StatusUnprocessableEntity, "INVALID_COMPENSATION", "One or more compensation fields are invalid."
	case errors.Is(err, compensation.ErrConflict):
		status, code, message = http.StatusConflict, "COMPENSATION_CONFLICT", "The compensation record changed. Refresh and retry."
	case errors.Is(err, abuse.ErrInvalid):
		status, code, message = http.StatusUnprocessableEntity, "INVALID_ABUSE_CONFIGURATION", "One or more abuse detector fields are invalid."
	case errors.Is(err, squadprofile.ErrInvalid):
		status, code, message = http.StatusUnprocessableEntity, "INVALID_SQUAD_PROFILE", "Squad profile fields are invalid."
	case errors.Is(err, database.ErrNotFound):
		status, code, message = http.StatusNotFound, "NOT_FOUND", "The requested record was not found."
	case errors.Is(err, database.ErrConflict), errors.Is(err, backup.ErrRestoreConflict):
		status, code, message = http.StatusConflict, "CONFLICT", "The operation conflicts with current state."
	case strings.Contains(strings.ToLower(err.Error()), "remnawave") || strings.Contains(strings.ToLower(err.Error()), "provider"):
		status, code, message = http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "The upstream operation failed."
	}
	s.writeError(w, r, status, code, message)
}

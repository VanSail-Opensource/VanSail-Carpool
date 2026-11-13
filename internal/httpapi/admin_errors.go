package httpapi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
)

func (s *Server) adminFailure(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusUnprocessableEntity, "ADMIN_OPERATION_FAILED", "The administrator operation could not be completed. Check the supplied fields."
	var storageFailure interface{ SQLState() string }
	switch {
	case errors.Is(err, backup.ErrBackupForbidden):
		status, code, message = http.StatusForbidden, "FORBIDDEN", "Administrator access is required."
	case errors.Is(err, database.ErrPaymentProfileInUse):
		status, code, message = http.StatusConflict, "PAYMENT_PROFILE_IN_USE", "Retained orders need this account for callbacks. Create a replacement payment profile and disable this one."
	case errors.Is(err, squadprofile.ErrInvalid):
		status, code, message = http.StatusUnprocessableEntity, "INVALID_SQUAD_PROFILE", "Squad profile fields are invalid."
	case errors.Is(err, squadprofile.ErrInvalidFeatures):
		status, code, message = http.StatusUnprocessableEntity, "INVALID_SQUAD_FEATURES", "Squad feature fields are invalid."
	case errors.Is(err, database.ErrNotFound):
		status, code, message = http.StatusNotFound, "NOT_FOUND", "The requested record was not found."
	case errors.Is(err, database.ErrOperationReconciliationRequired):
		status, code, message = http.StatusConflict, "OPERATION_RECONCILIATION_REQUIRED", "Reconcile the owning workflow before repairing this operation receipt."
	case errors.Is(err, database.ErrConflict), errors.Is(err, backup.ErrRestoreConflict):
		status, code, message = http.StatusConflict, "CONFLICT", "The operation conflicts with current state."
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.Is(err, sql.ErrConnDone), errors.Is(err, driver.ErrBadConn):
		status, code, message = http.StatusServiceUnavailable, "ADMIN_OPERATION_UNAVAILABLE", "The administrator operation is temporarily unavailable."
	case errors.As(err, &storageFailure):
		status, code, message = http.StatusInternalServerError, "ADMIN_STORAGE_FAILED", "The administrator operation could not be stored."
	case strings.Contains(strings.ToLower(err.Error()), "provider"):
		status, code, message = http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "The upstream operation failed."
	}
	s.writeError(w, r, status, code, message)
}

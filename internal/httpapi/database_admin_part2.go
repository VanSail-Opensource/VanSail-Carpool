package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/databaseadmin"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
	"strings"
)

func (h *DatabaseAdministrationHTTP) failure(w http.ResponseWriter, r *http.Request, err error) {
	var protection *databaseadmin.ProtectionError
	if errors.As(err, &protection) {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "DATABASE_PROTECTED", Message: "The table or field is protected.", Details: map[string]string{"reason": protection.Reason, "column": protection.Column, "table": protection.Table}})
		return
	}
	status, code, message := http.StatusUnprocessableEntity, "DATABASE_ADMIN_FAILED", "The database administration operation could not be completed."
	switch {
	case errors.Is(err, databaseadmin.ErrTableNotFound), errors.Is(err, databaseadmin.ErrRecordNotFound), errors.Is(err, backup.ErrBackupNotFound):
		status, code, message = http.StatusNotFound, "NOT_FOUND", "The requested table, record, backup, or restore was not found."
	case errors.Is(err, databaseadmin.ErrOptimisticConflict), errors.Is(err, databaseadmin.ErrReviewConflict), errors.Is(err, backup.ErrRestoreConflict):
		status, code, message = http.StatusConflict, "DATABASE_CONFLICT", "The record or review changed. Refresh, review the diff again, and retry."
	case errors.Is(err, databaseadmin.ErrInvalidValue), errors.Is(err, databaseadmin.ErrConfirmation):
		status, code, message = http.StatusUnprocessableEntity, "INVALID_DATABASE_MUTATION", err.Error()
	case strings.Contains(strings.ToLower(err.Error()), "backup") || strings.Contains(strings.ToLower(err.Error()), "restore"):
		status, code, message = http.StatusInternalServerError, "BACKUP_OPERATION_FAILED", "The verified backup operation could not be completed."
	}
	h.writeError(w, r, status, code, message)
}

func (h *DatabaseAdministrationHTTP) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, apiError{Code: code, Message: message, RequestID: middleware.GetReqID(r.Context())})
}

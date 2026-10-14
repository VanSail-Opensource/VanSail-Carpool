package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/databaseadmin"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
	"strings"
)

func mapRestoreOperation(job backup.RestoreJob) restoreOperationResponse {
	status := "staging"
	switch job.Status {
	case "ready", "applying":
		status = "restarting"
	case "complete":
		status = "complete"
	case "failed", "rolled_back":
		status = "failed"
	}
	var message *string
	if status == "failed" {
		value := "Restore did not complete. The original database remains available."
		message = &value
	}
	return restoreOperationResponse{ID: job.ID, BackupID: job.BackupID, Status: status, Error: message, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt}
}

func (h *DatabaseAdministrationHTTP) failure(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusUnprocessableEntity, "DATABASE_ADMIN_FAILED", "The database administration operation could not be completed."
	switch {
	case errors.Is(err, backup.ErrUploadTooLarge):
		status, code, message = http.StatusRequestEntityTooLarge, "BACKUP_TOO_LARGE", "The backup exceeds the configured upload limit."
	case errors.Is(err, backup.ErrUploadHashMismatch):
		status, code, message = http.StatusBadRequest, "BACKUP_HASH_MISMATCH", "The uploaded backup does not match its SHA-256 digest."
	case errors.Is(err, backup.ErrUploadConflict):
		status, code, message = http.StatusConflict, "BACKUP_UPLOAD_CONFLICT", "The upload key is already associated with different or unfinished content."
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

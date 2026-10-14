package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
)

func (h *DatabaseAdministrationHTTP) upload(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		h.writeError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Send an Idempotency-Key header containing 1 to 128 characters.")
		return
	}
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.writeError(w, r, http.StatusInternalServerError, "UPLOAD_UNAVAILABLE", "The upload connection could not be prepared.")
		return
	}
	if err := controller.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.writeError(w, r, http.StatusInternalServerError, "UPLOAD_UNAVAILABLE", "The upload connection could not be prepared.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.uploadMaxBytes+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_BACKUP_UPLOAD", "Use multipart form data with one SQLite file.")
		return
	}
	var candidate backup.UploadCandidate
	var expectedSHA string
	finalized := false
	defer func() {
		if candidate.ID == "" || finalized {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.backups.AbortUpload(ctx, candidate.ID, errors.New("backup upload request ended before publication"))
	}()
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			h.failure(w, r, nextErr)
			return
		}
		switch part.FormName() {
		case "file":
			if candidate.ID != "" || part.FileName() == "" {
				_ = part.Close()
				h.writeError(w, r, http.StatusBadRequest, "INVALID_BACKUP_UPLOAD", "Upload exactly one SQLite file.")
				return
			}
			candidate, err = h.backups.ReceiveUpload(r.Context(), currentUser(r).ID, key, part.FileName(), part, h.uploadMaxBytes)
		case "sha256":
			if expectedSHA != "" {
				err = errors.New("duplicate SHA-256 field")
			} else {
				expectedSHA, err = readBackupSHA(part)
			}
		default:
			err = errors.New("unexpected backup upload field")
		}
		closeErr := part.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			h.failure(w, r, err)
			return
		}
	}
	if candidate.ID == "" {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_BACKUP_UPLOAD", "A SQLite backup file is required.")
		return
	}
	run, err := h.backups.FinalizeUpload(r.Context(), candidate.ID, expectedSHA, h.migrations)
	if err != nil {
		h.failure(w, r, err)
		return
	}
	finalized = true
	writeJSON(w, http.StatusCreated, mapBackupRun(run))
}

func readBackupSHA(reader io.Reader) (string, error) {
	value, err := io.ReadAll(io.LimitReader(reader, 129))
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(value))
	if len(value) == 129 || len(trimmed) != 64 {
		return "", backup.ErrUploadHashMismatch
	}
	return trimmed, nil
}

package backup

import (
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const (
	// DefaultUploadMaxBytes is the default streamed backup upload cap (2 GiB).
	DefaultUploadMaxBytes  int64 = 2 << 30
	uploadStatusReceiving        = "receiving"
	uploadStatusValidating       = "validating"
	uploadStatusPublishing       = "publishing"
	uploadStatusComplete         = "complete"
	uploadStatusFailed           = "failed"
)

var (
	// ErrUploadTooLarge reports that the streamed body crossed the configured cap.
	ErrUploadTooLarge = errors.New("backup upload exceeds the configured limit")
	// ErrUploadHashMismatch reports a caller-supplied digest mismatch.
	ErrUploadHashMismatch = errors.New("backup upload SHA-256 does not match")
	// ErrUploadConflict reports an incompatible replay or unfinished intake.
	ErrUploadConflict = errors.New("backup upload conflicts with current state")
)

// UploadCandidate is a streamed file awaiting isolated SQLite validation.
type UploadCandidate struct {
	ID           string
	Backup       model.BackupRun
	ActualSHA256 string
	Replayed     bool
}

type uploadRecord struct {
	ID, BackupRunID, ActorUserID, IdempotencyKey string
	OriginalFilename                             string
	ExpectedSHA256, ActualSHA256                 string
	TemporaryPath, FinalPath, Status, LastError  string
	SizeBytes                                    int64
	CreatedAt, UpdatedAt                         time.Time
	CompletedAt                                  *time.Time
}

package backup

import (
	"context"
	"errors"
	"fmt"
	platformdatabase "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	markerSuffix = ".restore-pending.json"
	resultSuffix = ".restore-result.json"
)

// ErrRestoreConflict reports an invalid confirmation, incompatible snapshot,
// concurrent restore, or restore record that changed while being staged.
var ErrRestoreConflict = errors.New("restore conflicts with current state")

// RestoreJob describes a durable staged restore operation.
type RestoreJob struct {
	ID                 string     `json:"id"`
	BackupID           string     `json:"backupId"`
	ActorUserID        *string    `json:"actorUserId,omitempty"`
	RequestActorID     string     `json:"-"`
	IdempotencyKey     string     `json:"-"`
	RequestFingerprint string     `json:"-"`
	Status             string     `json:"status"`
	StagedPath         string     `json:"-"`
	RescuePath         string     `json:"-"`
	SourceSHA256       string     `json:"sourceSha256"`
	Error              string     `json:"error,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	CompletedAt        *time.Time `json:"completedAt,omitempty"`
}

type restoreMarker struct {
	Version            int       `json:"version"`
	JobID              string    `json:"jobId"`
	BackupID           string    `json:"backupId"`
	ActorUserID        *string   `json:"actorUserId,omitempty"`
	RequestActorID     string    `json:"requestActorId,omitempty"`
	IdempotencyKey     string    `json:"idempotencyKey,omitempty"`
	RequestFingerprint string    `json:"requestFingerprint,omitempty"`
	DatabasePath       string    `json:"databasePath"`
	StagedPath         string    `json:"stagedPath"`
	RescuePath         string    `json:"rescuePath"`
	SourcePath         string    `json:"sourcePath"`
	SourceSize         int64     `json:"sourceSize"`
	SourceSHA256       string    `json:"sourceSha256"`
	CreatedAt          time.Time `json:"createdAt"`
}

// StartupRestoreResult is written beside the database during the pre-open
// atomic swap and imported into restore_jobs after SQLite is opened.

type StartupRestoreResult struct {
	restoreMarker
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	CompletedAt time.Time `json:"completedAt"`
}

// StageRestore verifies a stored snapshot, copies it beside the live database,
// creates a verified rescue backup, and publishes a durable pre-open marker.
// No live database file is replaced by this method.

func (s *Service) StageRestore(ctx context.Context, backupID, actorUserID, idempotencyKey, reason, confirmation string, supportedMigrations []string) (RestoreJob, error) {
	if s.postgresURL != "" {
		return RestoreJob{}, fmt.Errorf("%w: PostgreSQL restore requires offline maintenance and recovery validation", ErrRestoreConflict)
	}
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()

	request, err := normalizeRestoreRequest(backupID, actorUserID, idempotencyKey, reason, confirmation)
	if err != nil {
		return RestoreJob{}, err
	}
	if replay, ok, err := s.restoreReplay(ctx, request); err != nil {
		return RestoreJob{}, err
	} else if ok {
		return replay, nil
	}
	if len(supportedMigrations) == 0 {
		return RestoreJob{}, fmt.Errorf("%w: migration allowlist is empty", ErrRestoreConflict)
	}
	if active, err := s.pendingMarkerExists(ctx); err != nil {
		return RestoreJob{}, err
	} else if active {
		return RestoreJob{}, fmt.Errorf("%w: another restore is already staged", ErrRestoreConflict)
	}

	download, err := s.OpenDownload(ctx, request.BackupID)
	if err != nil {
		return RestoreJob{}, err
	}
	defer func() { _ = download.File.Close() }()
	if request.Confirmation != "RESTORE "+download.Name {
		return RestoreJob{}, fmt.Errorf("%w: typed restore confirmation does not match", ErrRestoreConflict)
	}
	databasePath, err := s.mainDatabasePath(ctx)
	if err != nil {
		return RestoreJob{}, err
	}
	jobID, err := ids.New()
	if err != nil {
		return RestoreJob{}, fmt.Errorf("create restore identifier: %w", err)
	}
	now := s.now().UTC()
	stagePath := filepath.Join(filepath.Dir(databasePath), "."+filepath.Base(databasePath)+".restore-"+jobID+".stage")
	_, err = copyAndSync(download.File, stagePath)
	if err != nil {
		return RestoreJob{}, fmt.Errorf("stage restore snapshot: %w", err)
	}
	removeStage := true
	defer func() {
		if removeStage {
			_ = os.Remove(stagePath)
		}
	}()
	if err := verifySnapshot(ctx, stagePath, supportedMigrations); err != nil {
		return RestoreJob{}, fmt.Errorf("%w: %v", ErrRestoreConflict, err)
	}
	if err := platformdatabase.PrepareRestoreSnapshot(ctx, stagePath); err != nil {
		return RestoreJob{}, fmt.Errorf("%w: preflight staged snapshot: %v", ErrRestoreConflict, err)
	}
	if err := verifyCurrentSnapshot(ctx, stagePath, supportedMigrations); err != nil {
		return RestoreJob{}, fmt.Errorf("%w: verify migrated staged snapshot: %v", ErrRestoreConflict, err)
	}
	sourceHash, err := hashFile(stagePath)
	if err != nil {
		return RestoreJob{}, fmt.Errorf("hash prepared restore snapshot: %w", err)
	}

	rescue, err := s.Run(ctx)
	if err != nil {
		return RestoreJob{}, fmt.Errorf("create rescue backup: %w", err)
	}
	actor := nullableActor(request.ActorID)
	job := RestoreJob{
		ID: jobID, BackupID: request.BackupID, ActorUserID: actor, RequestActorID: request.ActorID,
		IdempotencyKey: request.IdempotencyKey, RequestFingerprint: request.Fingerprint,
		Status: "staging", StagedPath: stagePath, RescuePath: rescue.Path,
		SourceSHA256: sourceHash, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.insertRestoreJob(ctx, job); err != nil {
		return RestoreJob{}, err
	}
	marker := restoreMarker{
		Version: 3, JobID: jobID, BackupID: request.BackupID, ActorUserID: actor,
		RequestActorID: request.ActorID, IdempotencyKey: request.IdempotencyKey, RequestFingerprint: request.Fingerprint,
		DatabasePath: databasePath,
		StagedPath:   stagePath, RescuePath: rescue.Path, SourcePath: filepath.Join(s.directory, download.Name),
		SourceSize: download.Size, SourceSHA256: sourceHash, CreatedAt: now,
	}
	if err := writeJSONAtomic(markerPath(databasePath), marker); err != nil {
		_ = s.failRestoreJob(ctx, jobID, err)
		return RestoreJob{}, fmt.Errorf("publish restore marker: %w", err)
	}
	if err := s.markRestoreReady(ctx, job, request.Reason, rescue.ID); err != nil {
		_ = os.Remove(markerPath(databasePath))
		_ = s.failRestoreJob(ctx, jobID, err)
		return RestoreJob{}, err
	}
	removeStage = false
	job.Status = "ready"
	job.UpdatedAt = s.now().UTC()
	return job, nil
}

func nullableActor(actor string) *string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil
	}
	return &actor
}

func (s *Service) pendingMarkerExists(ctx context.Context) (bool, error) {
	if s.db == nil {
		return false, errors.New("backup service database is unavailable")
	}
	path, err := s.mainDatabasePath(ctx)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(markerPath(path))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("inspect restore marker: %w", err)
}

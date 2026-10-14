package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	_ "modernc.org/sqlite"
	"strings"
	"time"
)

func (s *Service) insertRestoreJob(ctx context.Context, job RestoreJob) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO restore_jobs(id,backup_run_id,actor_user_id,request_actor_id,idempotency_key,
		request_fingerprint,status,staged_path,rescue_path,source_sha256,error,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, job.ID, job.BackupID, job.ActorUserID, job.RequestActorID, job.IdempotencyKey,
		job.RequestFingerprint, job.Status, job.StagedPath, job.RescuePath, job.SourceSHA256, "",
		job.CreatedAt.Format(time.RFC3339Nano), job.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("record staged restore: %w", err)
	}
	return nil
}

func (s *Service) markRestoreReady(ctx context.Context, job RestoreJob, reason, rescueBackupID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restore audit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := s.now().UTC()
	result, err := tx.ExecContext(ctx, `UPDATE restore_jobs SET status='ready',updated_at=? WHERE id=? AND status='staging'`, now.Format(time.RFC3339Nano), job.ID)
	if err != nil {
		return fmt.Errorf("mark restore ready: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return fmt.Errorf("%w: restore record changed while staging", ErrRestoreConflict)
	}
	detail, err := json.Marshal(map[string]any{
		"backupId": job.BackupID, "rescueBackupId": rescueBackupID, "reason": reason,
		"sourceSha256": job.SourceSHA256, "warning": "direct restore bypasses domain synchronization hooks",
	})
	if err != nil {
		return fmt.Errorf("encode restore audit: %w", err)
	}
	auditID, err := ids.New()
	if err != nil {
		return fmt.Errorf("create restore audit identifier: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES(?,?,?,?,?,?,?)`,
		auditID, job.ActorUserID, "database_restore_staged", "restore_job", job.ID, string(detail), now.Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("record restore audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit restore staging: %w", err)
	}
	return nil
}

func (s *Service) failRestoreJob(ctx context.Context, id string, restoreErr error) error {
	message := strings.TrimSpace(restoreErr.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	_, err := s.db.ExecContext(ctx, `UPDATE restore_jobs SET status='failed',error=?,updated_at=?,completed_at=? WHERE id=?`,
		message, s.now().UTC().Format(time.RFC3339Nano), s.now().UTC().Format(time.RFC3339Nano), id)
	return err
}

// Restore returns one restore job by identifier.
func (s *Service) Restore(ctx context.Context, id string) (RestoreJob, error) {
	return scanRestoreJob(s.db.QueryRowContext(ctx, `SELECT `+restoreJobColumns+` FROM restore_jobs WHERE id=?`, id))
}

func (s *Service) restoreByRequestKey(ctx context.Context, actorID, key string) (RestoreJob, error) {
	return scanRestoreJob(s.db.QueryRowContext(ctx, `SELECT `+restoreJobColumns+`
		FROM restore_jobs WHERE request_actor_id=? AND idempotency_key=?`, actorID, key))
}

const restoreJobColumns = `id,backup_run_id,actor_user_id,request_actor_id,idempotency_key,request_fingerprint,
	status,staged_path,rescue_path,source_sha256,error,created_at,updated_at,completed_at`

type restoreJobScanner interface {
	Scan(...any) error
}

func scanRestoreJob(row restoreJobScanner) (RestoreJob, error) {
	var job RestoreJob
	var actor, completed sql.NullString
	var created, updated string
	err := row.Scan(&job.ID, &job.BackupID, &actor, &job.RequestActorID, &job.IdempotencyKey, &job.RequestFingerprint,
		&job.Status, &job.StagedPath, &job.RescuePath, &job.SourceSHA256, &job.Error, &created, &updated, &completed)
	if errors.Is(err, sql.ErrNoRows) {
		return RestoreJob{}, ErrBackupNotFound
	}
	if err != nil {
		return RestoreJob{}, fmt.Errorf("lookup restore: %w", err)
	}
	if actor.Valid {
		job.ActorUserID = &actor.String
	}
	job.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return RestoreJob{}, fmt.Errorf("parse restore creation time: %w", err)
	}
	job.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return RestoreJob{}, fmt.Errorf("parse restore update time: %w", err)
	}
	if completed.Valid {
		value, parseErr := time.Parse(time.RFC3339Nano, completed.String)
		if parseErr != nil {
			return RestoreJob{}, fmt.Errorf("parse restore completion time: %w", parseErr)
		}
		job.CompletedAt = &value
	}
	return job, nil
}

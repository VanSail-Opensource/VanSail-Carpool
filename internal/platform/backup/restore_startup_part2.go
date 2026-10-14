package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"os"
	"time"
)

func RecordStartupRestore(ctx context.Context, db *sql.DB, databasePath string) (*StartupRestoreResult, error) {
	result, err := readStartupResult(resultPath(databasePath))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return &result, fmt.Errorf("begin startup restore record: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stamp := result.CompletedAt.Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO backup_runs(id,path,size_bytes,status,error,created_at,completed_at)
		VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET path=excluded.path,size_bytes=excluded.size_bytes,status='complete',error='',completed_at=excluded.completed_at`,
		result.BackupID, result.SourcePath, result.SourceSize, "complete", "", result.CreatedAt.Format(time.RFC3339Nano), stamp); err != nil {
		return &result, fmt.Errorf("restore source backup record: %w", err)
	}
	var actor any
	if result.ActorUserID != nil {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=?`, *result.ActorUserID).Scan(&exists); err != nil {
			return &result, fmt.Errorf("verify restore actor: %w", err)
		}
		if exists > 0 {
			actor = *result.ActorUserID
		}
	}
	status := result.Status
	if status != "complete" && status != "failed" && status != "rolled_back" {
		status = "failed"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO restore_jobs(id,backup_run_id,actor_user_id,request_actor_id,idempotency_key,
		request_fingerprint,status,staged_path,rescue_path,source_sha256,error,created_at,updated_at,completed_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET actor_user_id=excluded.actor_user_id,
		request_actor_id=excluded.request_actor_id,idempotency_key=excluded.idempotency_key,
		request_fingerprint=excluded.request_fingerprint,status=excluded.status,error=excluded.error,
		updated_at=excluded.updated_at,completed_at=excluded.completed_at`,
		result.JobID, result.BackupID, actor, result.RequestActorID, result.IdempotencyKey, result.RequestFingerprint,
		status, result.StagedPath, result.RescuePath, result.SourceSHA256,
		truncateError(result.Error), result.CreatedAt.Format(time.RFC3339Nano), stamp, stamp); err != nil {
		return &result, fmt.Errorf("record startup restore job: %w", err)
	}
	auditID, err := ids.New()
	if err != nil {
		return &result, fmt.Errorf("create startup restore audit identifier: %w", err)
	}
	detail, _ := json.Marshal(map[string]any{"backupId": result.BackupID, "status": status, "error": truncateError(result.Error)})
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES(?,?,?,?,?,?,?)`,
		auditID, actor, "database_restore_"+status, "restore_job", result.JobID, string(detail), stamp); err != nil {
		return &result, fmt.Errorf("audit startup restore: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return &result, fmt.Errorf("commit startup restore record: %w", err)
	}
	if err := os.Remove(resultPath(databasePath)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return &result, fmt.Errorf("remove recorded restore result: %w", err)
	}
	return &result, nil
}

package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

const uploadSelect = `SELECT id,backup_run_id,actor_user_id,idempotency_key,expected_sha256,actual_sha256,
	temporary_path,final_path,size_bytes,status,last_error,created_at,updated_at,completed_at FROM backup_uploads`

func (s *Service) insertUpload(ctx context.Context, record uploadRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO backup_runs(id,path,status,source,actor_user_id,idempotency_key,
		original_filename,created_at) VALUES(?,?,'running','upload',?,?,?,?)`, record.BackupRunID, record.FinalPath,
		record.ActorUserID, record.IdempotencyKey, record.OriginalFilename, backupStamp(record.CreatedAt)); err != nil {
		return fmt.Errorf("create upload backup run: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO backup_uploads(id,backup_run_id,actor_user_id,idempotency_key,
		temporary_path,final_path,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, record.ID, record.BackupRunID,
		record.ActorUserID, record.IdempotencyKey, record.TemporaryPath, record.FinalPath, record.Status,
		backupStamp(record.CreatedAt), backupStamp(record.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create backup upload: %w", err)
	}
	return tx.Commit()
}

func (s *Service) markUploadReceived(ctx context.Context, record uploadRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE backup_uploads SET actual_sha256=?,size_bytes=?,status='validating',updated_at=?
		WHERE id=? AND status='receiving'`, record.ActualSHA256, record.SizeBytes, backupStamp(record.UpdatedAt), record.ID)
	if err != nil {
		return fmt.Errorf("record received backup upload: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return rowsErr
		}
		return ErrUploadConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE backup_runs SET size_bytes=?,sha256=?,request_fingerprint=? WHERE id=? AND status='running'`,
		record.SizeBytes, record.ActualSHA256, record.ActualSHA256, record.BackupRunID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) uploadByKey(ctx context.Context, actorID, key string) (uploadRecord, bool, error) {
	record, err := scanUpload(s.db.QueryRowContext(ctx, uploadSelect+` WHERE actor_user_id=? AND idempotency_key=?`, actorID, key))
	if errors.Is(err, sql.ErrNoRows) {
		return uploadRecord{}, false, nil
	}
	return record, err == nil, err
}

func (s *Service) uploadByID(ctx context.Context, id string) (uploadRecord, error) {
	return scanUpload(s.db.QueryRowContext(ctx, uploadSelect+` WHERE id=?`, id))
}

func scanUpload(row interface{ Scan(...any) error }) (uploadRecord, error) {
	var record uploadRecord
	var created, updated string
	var completed sql.NullString
	err := row.Scan(&record.ID, &record.BackupRunID, &record.ActorUserID, &record.IdempotencyKey,
		&record.ExpectedSHA256, &record.ActualSHA256, &record.TemporaryPath, &record.FinalPath, &record.SizeBytes,
		&record.Status, &record.LastError, &created, &updated, &completed)
	if err != nil {
		return uploadRecord{}, err
	}
	if record.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return uploadRecord{}, err
	}
	if record.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
		return uploadRecord{}, err
	}
	if completed.Valid {
		value, parseErr := time.Parse(time.RFC3339Nano, completed.String)
		if parseErr != nil {
			return uploadRecord{}, parseErr
		}
		record.CompletedAt = &value
	}
	return record, nil
}

func (s *Service) backupRun(ctx context.Context, id string) (model.BackupRun, error) {
	var run model.BackupRun
	var created string
	var completed sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,path,size_bytes,status,error,created_at,completed_at FROM backup_runs WHERE id=?`, id).
		Scan(&run.ID, &run.Path, &run.SizeBytes, &run.Status, &run.Error, &created, &completed)
	if err != nil {
		return model.BackupRun{}, err
	}
	if run.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return model.BackupRun{}, err
	}
	if completed.Valid {
		value, parseErr := time.Parse(time.RFC3339Nano, completed.String)
		if parseErr != nil {
			return model.BackupRun{}, parseErr
		}
		run.CompletedAt = &value
	}
	return run, nil
}

func backupStamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func newAuditID() (string, error) { return ids.New() }

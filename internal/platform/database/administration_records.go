package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"time"
)

// AppendAudit writes an immutable privileged action record.
func (s *Store) AppendAudit(ctx context.Context, actorUserID *string, action, targetType, targetID, detail string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	id, err := ids.New()
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin audit retention: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertAuditTx(ctx, tx, id, actorUserID, action, targetType, targetID, detail, now); err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit audit retention: %w", err)
	}
	return nil
}

func insertAuditTx(ctx context.Context, tx *sql.Tx, id string, actorUserID *string, action, targetType, targetID, detail string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES(?,?,?,?,?,?,?)`, id, actorUserID, action, targetType, targetID, detail, stamp(now))
	return err
}

// ListAuditEvents returns the newest administrative actions.
func (s *Store) ListAuditEvents(ctx context.Context, limit int) ([]model.AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,actor_user_id,action,target_type,target_id,detail,created_at FROM audit_events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	events := make([]model.AuditEvent, 0)
	for rows.Next() {
		var event model.AuditEvent
		var actor sql.NullString
		var created string
		if err := rows.Scan(&event.ID, &actor, &event.Action, &event.TargetType, &event.TargetID, &event.Detail, &created); err != nil {
			return nil, err
		}
		event.ActorUserID = nullableString(actor)
		event.CreatedAt, err = parseStamp(created)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// ListUsers returns local accounts for administration.
func (s *Store) ListUsers(ctx context.Context, limit int) ([]model.User, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, userSelect+` ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]model.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// StartBackupRun records a backup attempt.
func (s *Store) StartBackupRun(ctx context.Context, path string, now time.Time) (model.BackupRun, error) {
	id, err := ids.New()
	if err != nil {
		return model.BackupRun{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO backup_runs(id,path,status,created_at) VALUES(?,?,?,?)`, id, path, "running", stamp(now))
	if err != nil {
		return model.BackupRun{}, err
	}
	return model.BackupRun{ID: id, Path: path, Status: "running", CreatedAt: now}, nil
}

// CompleteBackupRun records verification outcome.
func (s *Store) CompleteBackupRun(ctx context.Context, id string, size int64, backupErr error, now time.Time) error {
	status, message := "complete", ""
	if backupErr != nil {
		status, message = "failed", sanitizeError(backupErr)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE backup_runs SET size_bytes=?,status=?,error=?,completed_at=? WHERE id=?`, size, status, message, stamp(now), id)
	return err
}

// ListBackupRuns returns backup history.
func (s *Store) ListBackupRuns(ctx context.Context, limit int) ([]model.BackupRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,path,size_bytes,status,error,created_at,completed_at FROM backup_runs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := make([]model.BackupRun, 0)
	for rows.Next() {
		var run model.BackupRun
		var created string
		var completed sql.NullString
		if err := rows.Scan(&run.ID, &run.Path, &run.SizeBytes, &run.Status, &run.Error, &created, &completed); err != nil {
			return nil, err
		}
		run.CreatedAt, err = parseStamp(created)
		if err != nil {
			return nil, err
		}
		if completed.Valid {
			value, parseErr := parseStamp(completed.String)
			if parseErr != nil {
				return nil, parseErr
			}
			run.CompletedAt = &value
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

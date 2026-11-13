package database

import (
	"context"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// StartAuditedBackupRun records the privileged attempt before an export starts.
func (s *Store) retryAttempt_StartAuditedBackupRun(ctx context.Context, path, actorID string, now time.Time) (model.BackupRun, error) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return model.BackupRun{}, ErrConflict
	}
	id, err := ids.New()
	if err != nil {
		return model.BackupRun{}, err
	}
	auditID, err := ids.New()
	if err != nil {
		return model.BackupRun{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.BackupRun{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return model.BackupRun{}, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO backup_runs(id,path,status,created_at) VALUES($1,$2,'running',$3)",
		id, path, stamp(now)); err != nil {
		return model.BackupRun{}, err
	}
	if err := insertAuditTx(ctx, tx, auditID, &actorID, "backup.create", "backup", id, `{"phase":"requested"}`, now); err != nil {
		return model.BackupRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.BackupRun{}, err
	}
	return model.BackupRun{ID: id, Path: path, Status: "running", CreatedAt: now}, nil
}

// Package backup creates database exports with bounded retention and contained downloads.
package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Repository interface {
	StartBackupRun(context.Context, string, time.Time) (model.BackupRun, error)
	CompleteBackupRun(context.Context, string, int64, error, time.Time) error
}

// Delete removes one backup file and its completed metadata while no restore is
// staging, ready, or restarting. The path is resolved against the configured
// backup directory and symlinks are rejected before any mutation.

func (s *Service) Delete(ctx context.Context, backupID, actorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var activeRestores int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM restore_jobs WHERE status IN ('staging','ready','applying')`).Scan(&activeRestores); err != nil {
		return fmt.Errorf("check active restores: %w", err)
	}
	if activeRestores != 0 {
		return ErrRestoreConflict
	}
	var path, status string
	if err := s.db.QueryRowContext(ctx, `SELECT path,status FROM backup_runs WHERE id=$1`, backupID).Scan(&path, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrBackupNotFound
		}
		return err
	}
	if status == "running" {
		return ErrRestoreConflict
	}
	resolved, err := s.resolveBackupFile(path, status == "failed")
	if err != nil {
		return err
	}
	staged := resolved + ".deleting-" + backupID
	fileExists := true
	if err := os.Rename(resolved, staged); err != nil {
		if errors.Is(err, os.ErrNotExist) && status == "failed" {
			fileExists = false
		} else {
			return fmt.Errorf("stage backup deletion: %w", err)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		if fileExists {
			_ = os.Rename(staged, resolved)
		}
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM restore_jobs WHERE backup_run_id=$1 AND status NOT IN ('staging','ready','applying')`, backupID); err != nil {
		if fileExists {
			_ = os.Rename(staged, resolved)
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM backup_runs WHERE id=$1`, backupID); err != nil {
		if fileExists {
			_ = os.Rename(staged, resolved)
		}
		return err
	}
	auditID, err := ids.New()
	if err != nil {
		if fileExists {
			_ = os.Rename(staged, resolved)
		}
		return err
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		auditID, actorID, "backup.delete", "backup", backupID, `{"path":"redacted"}`, now); err != nil {
		if fileExists {
			_ = os.Rename(staged, resolved)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		if fileExists {
			_ = os.Rename(staged, resolved)
		}
		return err
	}
	if fileExists {
		if err := os.Remove(staged); err != nil {
			return fmt.Errorf("remove deleted backup file: %w", err)
		}
	}
	return nil
}

func (s *Service) resolveBackupFile(path string, allowMissing bool) (string, error) {
	target, err := s.managedPath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(target)
	if err != nil {
		if allowMissing && errors.Is(err, os.ErrNotExist) {
			return target, nil
		}
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("backup path is not a regular file")
	}
	return target, nil
}

// Service owns backend-specific backup and retention lifecycles.

type Service struct {
	postgresURL string // Secret; never persisted in backup metadata or logs.
	db          *sql.DB
	repository  Repository
	directory   string
	retention   time.Duration
	now         func() time.Time
	mu          sync.Mutex
}

// NewService creates a backup service.

func NewService(db *sql.DB, repository Repository, directory string, retention time.Duration) *Service {
	return &Service{
		db: db, repository: repository, directory: filepath.Clean(directory), retention: retention,
		now: time.Now,
	}
}

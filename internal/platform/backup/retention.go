package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type expiredBackupRun struct {
	id   string
	path string
}

func (s *Service) removeExpiredRuns(ctx context.Context, cutoff time.Time) error {
	if s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT run.id,run.path FROM backup_runs run
		WHERE run.status IN ('complete','failed') AND COALESCE(run.completed_at,run.created_at)<?
		AND NOT EXISTS (SELECT 1 FROM restore_jobs restore WHERE restore.backup_run_id=run.id
			AND restore.status IN ('staging','ready','applying')) ORDER BY run.completed_at,run.id`, cutoff.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("list expired backups: %w", err)
	}
	defer rows.Close()
	runs := []expiredBackupRun{}
	for rows.Next() {
		var run expiredBackupRun
		if err = rows.Scan(&run.id, &run.path); err != nil {
			return fmt.Errorf("scan expired backup: %w", err)
		}
		runs = append(runs, run)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate expired backups: %w", err)
	}
	if err = rows.Close(); err != nil {
		return fmt.Errorf("close expired backup list: %w", err)
	}
	for _, run := range runs {
		if err = s.removeExpiredRun(ctx, run); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) removeExpiredRun(ctx context.Context, run expiredBackupRun) error {
	path, err := s.resolveBackupFile(run.path, true)
	if err != nil {
		return fmt.Errorf("resolve expired backup %q: %w", run.id, err)
	}
	if !isPublishedBackup(filepath.Base(path)) {
		return fmt.Errorf("expired backup %q has an unrecognized filename", run.id)
	}
	staged := path + ".deleting-" + run.id
	fileExists := true
	if err = os.Rename(path, staged); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stage expired backup %q: %w", run.id, err)
		}
		fileExists = false
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		if fileExists {
			_ = os.Rename(staged, path)
		}
		return fmt.Errorf("begin expired backup deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `DELETE FROM restore_jobs WHERE backup_run_id=? AND status NOT IN ('staging','ready','applying')`, run.id); err != nil {
		return s.restoreExpiredBackup(path, staged, fileExists, fmt.Errorf("delete expired backup restores: %w", err))
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM backup_runs WHERE id=? AND status IN ('complete','failed')`, run.id)
	if err != nil {
		return s.restoreExpiredBackup(path, staged, fileExists, fmt.Errorf("delete expired backup record: %w", err))
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return s.restoreExpiredBackup(path, staged, fileExists, err)
	}
	if changed != 1 {
		return s.restoreExpiredBackup(path, staged, fileExists, ErrBackupNotFound)
	}
	if err = tx.Commit(); err != nil {
		return s.restoreExpiredBackup(path, staged, fileExists, fmt.Errorf("commit expired backup deletion: %w", err))
	}
	if fileExists {
		if err = os.Remove(staged); err != nil {
			return fmt.Errorf("remove expired backup file: %w", err)
		}
	}
	return nil
}

func (s *Service) hasActiveRestore(ctx context.Context, path string) (bool, error) {
	if s.db == nil {
		return false, nil
	}
	var active int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs run JOIN restore_jobs restore ON restore.backup_run_id=run.id
		WHERE run.path=? AND restore.status IN ('staging','ready','applying')`, path).Scan(&active)
	if err != nil {
		return false, fmt.Errorf("check active backup restore: %w", err)
	}
	return active != 0, nil
}

func (s *Service) restoreExpiredBackup(path, staged string, exists bool, cause error) error {
	if exists {
		_ = os.Rename(staged, path)
	}
	return cause
}

func isPublishedBackup(name string) bool {
	return strings.HasPrefix(name, "tx-carpool-") && (strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".dump"))
}

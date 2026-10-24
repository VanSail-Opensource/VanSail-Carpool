package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) Run(ctx context.Context) (model.BackupRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	if err := os.MkdirAll(s.directory, 0o750); err != nil {
		return model.BackupRun{}, fmt.Errorf("create backup directory: %w", err)
	}
	name := "tx-carpool-" + now.Format("20060102T150405.000000000Z") + ".db"
	if s.postgresURL != "" {
		name = "tx-carpool-" + now.Format("20060102T150405.000000000Z") + ".dump"
	}
	finalPath := filepath.Join(s.directory, name)
	temporaryPath := filepath.Join(s.directory, "."+name+".tmp")
	run, err := s.repository.StartBackupRun(ctx, finalPath, now)
	if err != nil {
		return model.BackupRun{}, err
	}
	backupErr := s.createAndVerify(ctx, temporaryPath, finalPath)
	var size int64
	if backupErr == nil {
		if info, statErr := os.Stat(finalPath); statErr != nil {
			backupErr = statErr
		} else {
			size = info.Size()
		}
	}
	if completeErr := s.repository.CompleteBackupRun(ctx, run.ID, size, backupErr, s.now().UTC()); completeErr != nil && backupErr == nil {
		backupErr = completeErr
	}
	if backupErr != nil {
		_ = os.Remove(temporaryPath)
		run.Status = "failed"
		run.Error = backupErr.Error()
		return run, backupErr
	}
	run.Path, run.SizeBytes, run.Status = finalPath, size, "complete"
	completed := s.now().UTC()
	run.CompletedAt = &completed
	if err := s.removeExpired(ctx); err != nil {
		return run, fmt.Errorf("backup complete but retention failed: %w", err)
	}
	return run, nil
}

func (s *Service) createAndVerify(ctx context.Context, temporaryPath, finalPath string) error {
	if s.postgresURL != "" {
		return s.exportPostgres(ctx, temporaryPath, finalPath)
	}
	if _, err := os.Stat(temporaryPath); err == nil {
		if removeErr := os.Remove(temporaryPath); removeErr != nil {
			return fmt.Errorf("remove stale temporary backup: %w", removeErr)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
		return fmt.Errorf("checkpoint before SQLite backup: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, temporaryPath); err != nil {
		return fmt.Errorf("create online SQLite backup: %w", err)
	}
	backupDB, err := sql.Open("sqlite", "file:"+filepath.ToSlash(temporaryPath)+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return fmt.Errorf("open backup for verification: %w", err)
	}
	var integrity string
	verificationErr := backupDB.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity)
	closeErr := backupDB.Close()
	if verificationErr != nil {
		return fmt.Errorf("verify SQLite backup: %w", verificationErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close verified SQLite backup: %w", closeErr)
	}
	if integrity != "ok" {
		return fmt.Errorf("SQLite backup integrity result: %s", integrity)
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return fmt.Errorf("publish backup: %w", err)
	}
	return nil
}

// RemoveExpired deletes expired managed backup files and their history.
func (s *Service) RemoveExpired() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeExpired(context.Background())
}

func (s *Service) removeExpired(ctx context.Context) error {
	if err := s.removeExpiredRuns(ctx, s.now().UTC().Add(-s.retention)); err != nil {
		return err
	}
	return s.removeExpiredFiles(ctx)
}

func (s *Service) removeExpiredFiles(ctx context.Context) error {
	entries, err := os.ReadDir(s.directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	cutoff := s.now().UTC().Add(-s.retention)
	for _, entry := range entries {
		name := entry.Name()
		if s.postgresURL != "" && !strings.HasSuffix(name, ".dump") && !strings.Contains(name, ".dump.") {
			continue
		}
		published := strings.HasPrefix(name, "tx-carpool-") && (strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".dump"))
		crashTemporary := strings.HasPrefix(name, ".tx-carpool-") &&
			(strings.HasSuffix(name, ".db.tmp") || strings.Contains(name, ".db.deleting-") || strings.HasSuffix(name, ".dump.tmp") || strings.Contains(name, ".dump.deleting-"))
		if entry.IsDir() || (!published && !crashTemporary) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.ModTime().UTC().Before(cutoff) {
			protected, protectErr := s.hasActiveRestore(ctx, filepath.Join(s.directory, name))
			if protectErr != nil {
				return protectErr
			}
			if protected {
				continue
			}
			if err := os.Remove(filepath.Join(s.directory, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

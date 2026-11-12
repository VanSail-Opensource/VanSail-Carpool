package backup

import (
	"context"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Service) Run(ctx context.Context) (model.BackupRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	if err := os.MkdirAll(s.directory, 0o750); err != nil {
		return model.BackupRun{}, fmt.Errorf("create backup directory: %w", err)
	}
	name := "tx-carpool-" + now.Format("20060102T150405.000000000Z") + ".dump"
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
	// A canceled export must still record its outcome before joined worker
	// shutdown closes the database; otherwise the run remains running forever.
	completionCtx, stopCompletion := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	completeErr := s.repository.CompleteBackupRun(completionCtx, run.ID, size, backupErr, s.now().UTC())
	stopCompletion()
	backupErr = errors.Join(backupErr, completeErr)
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
	if s.postgresURL == "" {
		return fmt.Errorf("PostgreSQL backup connection is not configured")
	}
	return s.exportPostgres(ctx, temporaryPath, finalPath)
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
		deletionSource := deletionSourceName(name)
		crashTemporary := strings.HasPrefix(name, ".tx-carpool-") &&
			(strings.HasSuffix(name, ".db.tmp") || strings.HasSuffix(name, ".dump.tmp"))
		crashTemporary = crashTemporary || deletionSource != ""
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
			protectedName := name
			if deletionSource != "" {
				protectedName = deletionSource
			}
			protected, protectErr := s.hasActiveRestore(ctx, filepath.Join(s.directory, protectedName))
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

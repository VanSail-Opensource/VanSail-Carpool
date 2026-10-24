package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// AbortUpload removes a received candidate that did not reach publication.
func (s *Service) AbortUpload(ctx context.Context, candidateID string, cause error) error {
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.uploadByID(ctx, candidateID)
	if err != nil {
		return err
	}
	if record.Status == uploadStatusComplete || record.Status == uploadStatusPublishing {
		return nil
	}
	if cause == nil {
		cause = errors.New("backup upload was not finalized")
	}
	return s.failUploadLocked(ctx, record, cause)
}

// ReconcileUploads resolves files left between durable publication phases.
func (s *Service) ReconcileUploads(ctx context.Context, supportedMigrations []string) error {
	if s.postgresURL != "" {
		return nil
	}
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.QueryContext(ctx, uploadSelect+` WHERE status IN ('receiving','validating','publishing') ORDER BY created_at`)
	if err != nil {
		return fmt.Errorf("list interrupted backup uploads: %w", err)
	}
	records := make([]uploadRecord, 0)
	for rows.Next() {
		record, scanErr := scanUpload(rows)
		if scanErr != nil {
			_ = rows.Close()
			return scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, record := range records {
		if record.Status != uploadStatusPublishing {
			if err := s.failUploadLocked(ctx, record, errors.New("backup upload interrupted before publication")); err != nil {
				return err
			}
			continue
		}
		if err := s.reconcilePublishedUpload(ctx, record, supportedMigrations); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reconcilePublishedUpload(ctx context.Context, record uploadRecord, migrations []string) error {
	finalPath, finalErr := s.containedUploadPath(record.FinalPath)
	if finalErr != nil {
		return finalErr
	}
	if _, err := os.Lstat(finalPath); errors.Is(err, os.ErrNotExist) {
		temporary, tempErr := s.safeUploadPath(record.TemporaryPath, true)
		if tempErr != nil {
			return s.failUploadLocked(ctx, record, tempErr)
		}
		if err := verifyReceivedUpload(ctx, temporary, record, migrations); err != nil {
			return s.failUploadLocked(ctx, record, err)
		}
		if err := os.Rename(temporary, finalPath); err != nil {
			return fmt.Errorf("recover uploaded backup publication: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect published upload: %w", err)
	}
	if err := verifyReceivedUpload(ctx, finalPath, record, migrations); err != nil {
		_ = os.Remove(finalPath)
		return s.failUploadLocked(ctx, record, err)
	}
	_, err := s.completeUploadLocked(ctx, record)
	return err
}

func (s *Service) failUploadLocked(ctx context.Context, record uploadRecord, cause error) error {
	if record.Status != uploadStatusPublishing {
		if path, err := s.containedUploadPath(record.TemporaryPath); err == nil {
			_ = os.Remove(path)
		}
	}
	now := s.now().UTC()
	message := truncateError(cause.Error())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE backup_uploads SET status='failed',last_error=?,updated_at=?,completed_at=?
		WHERE id=? AND status<>'complete'`, message, backupStamp(now), backupStamp(now), record.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE backup_runs SET status='failed',error=?,completed_at=? WHERE id=? AND status='running'`,
		message, backupStamp(now), record.BackupRunID); err != nil {
		return err
	}
	return tx.Commit()
}

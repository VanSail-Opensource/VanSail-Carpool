package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// FinalizeUpload verifies and atomically publishes a fully received candidate.
func (s *Service) FinalizeUpload(ctx context.Context, candidateID, expectedSHA256 string, supportedMigrations []string) (model.BackupRun, error) {
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.uploadByID(ctx, strings.TrimSpace(candidateID))
	if err != nil {
		return model.BackupRun{}, err
	}
	expectedSHA256, err = normalizeSHA256(expectedSHA256)
	if err != nil {
		return model.BackupRun{}, err
	}
	if record.Status == uploadStatusComplete {
		if expectedSHA256 != "" && expectedSHA256 != record.ActualSHA256 {
			return model.BackupRun{}, ErrUploadHashMismatch
		}
		return s.backupRun(ctx, record.BackupRunID)
	}
	if record.Status != uploadStatusValidating || len(supportedMigrations) == 0 {
		return model.BackupRun{}, ErrUploadConflict
	}
	if expectedSHA256 != "" && expectedSHA256 != record.ActualSHA256 {
		_ = s.failUploadLocked(ctx, record, ErrUploadHashMismatch)
		return model.BackupRun{}, ErrUploadHashMismatch
	}
	temporary, err := s.safeUploadPath(record.TemporaryPath, true)
	if err != nil {
		_ = s.failUploadLocked(ctx, record, err)
		return model.BackupRun{}, err
	}
	if err := verifyReceivedUpload(ctx, temporary, record, supportedMigrations); err != nil {
		_ = s.failUploadLocked(ctx, record, err)
		return model.BackupRun{}, err
	}
	now := s.now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE backup_uploads SET expected_sha256=?,status='publishing',updated_at=?
		WHERE id=? AND status='validating'`, expectedSHA256, backupStamp(now), record.ID)
	if err != nil {
		return model.BackupRun{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return model.BackupRun{}, rowsErr
		}
		return model.BackupRun{}, ErrUploadConflict
	}
	record.ExpectedSHA256, record.Status, record.UpdatedAt = expectedSHA256, uploadStatusPublishing, now
	finalPath, err := s.safeUploadDestination(record.FinalPath)
	if err != nil {
		return model.BackupRun{}, err
	}
	if err := os.Rename(temporary, finalPath); err != nil {
		return model.BackupRun{}, fmt.Errorf("publish uploaded backup: %w", err)
	}
	return s.completeUploadLocked(ctx, record)
}

func verifyReceivedUpload(ctx context.Context, path string, record uploadRecord, migrations []string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat uploaded backup: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != record.SizeBytes {
		return errors.New("uploaded backup size or file type changed")
	}
	digest, err := hashFile(path)
	if err != nil {
		return fmt.Errorf("hash uploaded backup: %w", err)
	}
	if digest != record.ActualSHA256 {
		return ErrUploadHashMismatch
	}
	if err := verifySnapshot(ctx, path, migrations); err != nil {
		return fmt.Errorf("validate uploaded SQLite snapshot: %w", err)
	}
	return nil
}

func normalizeSHA256(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return "", ErrUploadHashMismatch
	}
	return value, nil
}

func (s *Service) completeUploadLocked(ctx context.Context, record uploadRecord) (model.BackupRun, error) {
	now := s.now().UTC()
	auditID, err := newAuditID()
	if err != nil {
		return model.BackupRun{}, err
	}
	detail, err := json.Marshal(map[string]any{"sha256": record.ActualSHA256, "sizeBytes": record.SizeBytes})
	if err != nil {
		return model.BackupRun{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.BackupRun{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE backup_runs SET size_bytes=?,status='complete',error='',completed_at=? WHERE id=?`,
		record.SizeBytes, backupStamp(now), record.BackupRunID); err != nil {
		return model.BackupRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE backup_uploads SET status='complete',last_error='',updated_at=?,completed_at=?
		WHERE id=? AND status='publishing'`, backupStamp(now), backupStamp(now), record.ID); err != nil {
		return model.BackupRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
		VALUES(?,?,'backup.upload','backup',?,?,?)`, auditID, record.ActorUserID, record.BackupRunID, string(detail), backupStamp(now)); err != nil {
		return model.BackupRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.BackupRun{}, err
	}
	return s.backupRun(ctx, record.BackupRunID)
}

func (s *Service) safeUploadDestination(path string) (string, error) {
	clean, err := s.containedUploadPath(path)
	if err != nil {
		return "", err
	}
	name := filepath.Base(clean)
	if !strings.HasPrefix(name, "tx-carpool-upload-") || !strings.HasSuffix(name, ".db") {
		return "", errors.New("uploaded backup destination is not recognized")
	}
	if _, err := os.Lstat(clean); err == nil || !errors.Is(err, os.ErrNotExist) {
		return "", ErrUploadConflict
	}
	return clean, nil
}

func (s *Service) safeUploadPath(path string, temporary bool) (string, error) {
	clean, err := s.containedUploadPath(path)
	if err != nil {
		return "", err
	}
	if temporary && (!strings.HasPrefix(filepath.Base(clean), ".tx-carpool-upload-") || !strings.HasSuffix(clean, ".db.uploading")) {
		return "", errors.New("upload staging filename is not recognized")
	}
	info, err := os.Lstat(clean)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("upload staging file is not a regular contained file")
	}
	return clean, nil
}

func (s *Service) containedUploadPath(path string) (string, error) {
	directory, err := filepath.Abs(s.directory)
	if err != nil {
		return "", err
	}
	clean, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(directory, clean)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("upload path escapes the configured backup directory")
	}
	return clean, nil
}

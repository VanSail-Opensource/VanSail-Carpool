package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// ReceiveUpload streams one bounded candidate to private storage and hashes it.
func (s *Service) ReceiveUpload(ctx context.Context, actorID, idempotencyKey, originalFilename string, source io.Reader, maximumBytes int64) (UploadCandidate, error) {
	actorID, idempotencyKey = strings.TrimSpace(actorID), strings.TrimSpace(idempotencyKey)
	if actorID == "" || idempotencyKey == "" || len(idempotencyKey) > 128 || source == nil {
		return UploadCandidate{}, ErrUploadConflict
	}
	if maximumBytes <= 0 {
		maximumBytes = DefaultUploadMaxBytes
	}
	originalFilename = filepath.Base(strings.TrimSpace(originalFilename))
	if originalFilename == "." || originalFilename == "" || len(originalFilename) > 255 {
		return UploadCandidate{}, ErrUploadConflict
	}
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, found, err := s.uploadByKey(ctx, actorID, idempotencyKey); err != nil {
		return UploadCandidate{}, err
	} else if found {
		if existing.Status != uploadStatusComplete {
			return UploadCandidate{}, ErrUploadConflict
		}
		size, digest, hashErr := hashUpload(source, maximumBytes)
		if hashErr != nil {
			return UploadCandidate{}, hashErr
		}
		if size != existing.SizeBytes || digest != existing.ActualSHA256 {
			return UploadCandidate{}, ErrUploadConflict
		}
		run, err := s.backupRun(ctx, existing.BackupRunID)
		return UploadCandidate{ID: existing.ID, Backup: run, ActualSHA256: existing.ActualSHA256, Replayed: true}, err
	}
	if err := os.MkdirAll(s.directory, 0o750); err != nil {
		return UploadCandidate{}, fmt.Errorf("create backup directory: %w", err)
	}
	uploadID, err := ids.New()
	if err != nil {
		return UploadCandidate{}, err
	}
	runID, err := ids.New()
	if err != nil {
		return UploadCandidate{}, err
	}
	now := s.now().UTC()
	name := "tx-carpool-upload-" + now.Format("20060102T150405.000000000Z") + "-" + uploadID + ".db"
	record := uploadRecord{ID: uploadID, BackupRunID: runID, ActorUserID: actorID, IdempotencyKey: idempotencyKey,
		OriginalFilename: originalFilename,
		TemporaryPath:    filepath.Join(s.directory, "."+name+".uploading"), FinalPath: filepath.Join(s.directory, name),
		Status: uploadStatusReceiving, CreatedAt: now, UpdatedAt: now}
	if err := s.insertUpload(ctx, record); err != nil {
		return UploadCandidate{}, err
	}
	size, digest, err := streamUpload(record.TemporaryPath, source, maximumBytes)
	if err != nil {
		_ = s.failUploadLocked(ctx, record, err)
		return UploadCandidate{}, err
	}
	record.SizeBytes, record.ActualSHA256, record.Status, record.UpdatedAt = size, digest, uploadStatusValidating, s.now().UTC()
	if err := s.markUploadReceived(ctx, record); err != nil {
		return UploadCandidate{}, err
	}
	run := model.BackupRun{ID: runID, Path: record.FinalPath, SizeBytes: size, Status: "running", CreatedAt: now}
	return UploadCandidate{ID: uploadID, Backup: run, ActualSHA256: digest}, nil
}

func hashUpload(source io.Reader, maximum int64) (int64, string, error) {
	digest := sha256.New()
	size, err := io.Copy(digest, io.LimitReader(source, maximum+1))
	if err != nil {
		return 0, "", fmt.Errorf("hash replayed backup upload: %w", err)
	}
	if size > maximum {
		return 0, "", ErrUploadTooLarge
	}
	if size == 0 {
		return 0, "", ErrUploadConflict
	}
	return size, hex.EncodeToString(digest.Sum(nil)), nil
}

func streamUpload(path string, source io.Reader, maximum int64) (size int64, digest string, returnedErr error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, "", fmt.Errorf("create upload staging file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); returnedErr == nil && closeErr != nil {
			returnedErr = fmt.Errorf("close upload staging file: %w", closeErr)
		}
	}()
	hash := sha256.New()
	size, err = io.Copy(io.MultiWriter(file, hash), io.LimitReader(source, maximum+1))
	if err != nil {
		return 0, "", fmt.Errorf("stream backup upload: %w", err)
	}
	if size > maximum {
		return 0, "", ErrUploadTooLarge
	}
	if size == 0 {
		return 0, "", ErrUploadConflict
	}
	if err := file.Sync(); err != nil {
		return 0, "", fmt.Errorf("sync upload staging file: %w", err)
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

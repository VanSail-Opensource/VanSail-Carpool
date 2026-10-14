package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

const connectionScanSelect = `SELECT id,user_id,idempotency_key,request_fingerprint,provider_job_id,status,
	progress_percent,error_code,created_at,updated_at,completed_at,expires_at FROM connection_scans`

// UpdateConnectionScan advances progress without permitting job identity changes or regressions.
func (s *Store) UpdateConnectionScan(ctx context.Context, scanID string, update providerops.ConnectionScanUpdate, now time.Time) (providerops.ConnectionScan, error) {
	update.ProviderJobID = strings.TrimSpace(update.ProviderJobID)
	update.ErrorCode = operationCode(update.ErrorCode)
	if update.ProgressPercent < 0 || update.ProgressPercent > 100 ||
		(update.Status != providerops.StatusProcessing && update.Status != providerops.StatusSucceeded &&
			update.Status != providerops.StatusFailed && update.Status != providerops.StatusPendingReview) {
		return providerops.ConnectionScan{}, ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.ConnectionScan{}, fmt.Errorf("begin connection scan update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	current, err := scanConnectionScan(tx.QueryRowContext(ctx, connectionScanSelect+` WHERE id=?`, scanID))
	if err != nil {
		return providerops.ConnectionScan{}, err
	}
	if current.Status == providerops.StatusSucceeded || current.Status == providerops.StatusFailed || current.Status == providerops.StatusPendingReview {
		if current.Status == update.Status && (update.ProviderJobID == "" || current.ProviderJobID == update.ProviderJobID) && current.ProgressPercent == update.ProgressPercent {
			return current, nil
		}
		return providerops.ConnectionScan{}, ErrConflict
	}
	if current.ProviderJobID != "" && update.ProviderJobID != "" && current.ProviderJobID != update.ProviderJobID {
		return providerops.ConnectionScan{}, ErrConflict
	}
	providerJobID := current.ProviderJobID
	if providerJobID == "" {
		providerJobID = update.ProviderJobID
	}
	if update.ProgressPercent < current.ProgressPercent {
		return providerops.ConnectionScan{}, ErrConflict
	}
	if current.Status == providerops.StatusQueued && update.Status != providerops.StatusProcessing && update.Status != providerops.StatusFailed {
		return providerops.ConnectionScan{}, ErrConflict
	}
	if update.Status == providerops.StatusSucceeded {
		if providerJobID == "" {
			return providerops.ConnectionScan{}, ErrConflict
		}
		update.ProgressPercent = 100
	}
	var completed any
	if update.Status == providerops.StatusSucceeded || update.Status == providerops.StatusFailed || update.Status == providerops.StatusPendingReview {
		completed = stamp(now)
	}
	_, err = tx.ExecContext(ctx, `UPDATE connection_scans SET provider_job_id=?,status=?,progress_percent=?,
		error_code=?,completed_at=?,updated_at=? WHERE id=?`, providerJobID, update.Status, update.ProgressPercent,
		update.ErrorCode, completed, stamp(now), scanID)
	if err != nil {
		return providerops.ConnectionScan{}, fmt.Errorf("update connection scan: %w", err)
	}
	scan, err := scanConnectionScan(tx.QueryRowContext(ctx, connectionScanSelect+` WHERE id=?`, scanID))
	if err != nil {
		return providerops.ConnectionScan{}, err
	}
	if err := tx.Commit(); err != nil {
		return providerops.ConnectionScan{}, fmt.Errorf("commit connection scan update: %w", err)
	}
	return scan, nil
}

func scanConnectionScan(row rowScanner) (providerops.ConnectionScan, error) {
	var scan providerops.ConnectionScan
	var status, created, updated, expires string
	var completed sql.NullString
	err := row.Scan(&scan.ID, &scan.UserID, &scan.IdempotencyKey, &scan.RequestFingerprint,
		&scan.ProviderJobID, &status, &scan.ProgressPercent, &scan.ErrorCode, &created, &updated, &completed, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return providerops.ConnectionScan{}, ErrNotFound
	}
	if err != nil {
		return providerops.ConnectionScan{}, fmt.Errorf("scan connection scan: %w", err)
	}
	scan.Status = providerops.Status(status)
	if scan.CreatedAt, err = parseStamp(created); err == nil {
		scan.UpdatedAt, err = parseStamp(updated)
	}
	if err == nil {
		scan.CompletedAt, err = parseOptionalStamp(completed)
	}
	if err == nil {
		scan.ExpiresAt, err = parseStamp(expires)
	}
	return scan, err
}

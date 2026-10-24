package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// CreateConnectionScan persists or replays metadata and queues the provider request atomically.
func (s *Store) retryAttempt_CreateConnectionScan(ctx context.Context, input providerops.ConnectionScanInput, now time.Time) (providerops.ConnectionScan, bool, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.RequestFingerprint = strings.TrimSpace(input.RequestFingerprint)
	input.ProviderJobID = strings.TrimSpace(input.ProviderJobID)
	if input.UserID == "" || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 ||
		len(input.RequestFingerprint) < 16 || len(input.RequestFingerprint) > 128 || !input.ExpiresAt.After(now) {
		return providerops.ConnectionScan{}, false, ErrConflict
	}
	now = now.UTC()
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.ConnectionScan{}, false, fmt.Errorf("begin connection scan: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	existing, loadErr := scanConnectionScan(tx.QueryRowContext(ctx, connectionScanSelect+
		` WHERE user_id=? AND idempotency_key=?`, input.UserID, input.IdempotencyKey))
	if loadErr == nil {
		if existing.RequestFingerprint != input.RequestFingerprint {
			return providerops.ConnectionScan{}, false, ErrConflict
		}
		return existing, true, nil
	}
	if !errors.Is(loadErr, ErrNotFound) {
		return providerops.ConnectionScan{}, false, loadErr
	}
	scanID, err := ids.New()
	if err != nil {
		return providerops.ConnectionScan{}, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO connection_scans(id,user_id,idempotency_key,request_fingerprint,
		provider_job_id,status,progress_percent,created_at,updated_at,expires_at)
		VALUES(?,?,?,?,?,'queued',0,?,?,?)`, scanID, input.UserID, input.IdempotencyKey,
		input.RequestFingerprint, input.ProviderJobID, stamp(now), stamp(now), stamp(input.ExpiresAt))
	if err != nil {
		return providerops.ConnectionScan{}, false, fmt.Errorf("insert connection scan: %w", err)
	}
	payload, err := json.Marshal(map[string]string{"scanId": scanID})
	if err != nil {
		return providerops.ConnectionScan{}, false, fmt.Errorf("encode connection scan job: %w", err)
	}
	if err := insertOutboxTx(ctx, tx, "connection_scan_request", string(payload), now, now); err != nil {
		return providerops.ConnectionScan{}, false, err
	}
	scan, err := scanConnectionScan(tx.QueryRowContext(ctx, connectionScanSelect+` WHERE id=?`, scanID))
	if err != nil {
		return providerops.ConnectionScan{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return providerops.ConnectionScan{}, false, fmt.Errorf("commit connection scan: %w", err)
	}
	return scan, false, nil
}

// CreateOrReplayConnectionScan is an explicit alias for member services.
func (s *Store) CreateOrReplayConnectionScan(ctx context.Context, input providerops.ConnectionScanInput, now time.Time) (providerops.ConnectionScan, bool, error) {
	return s.CreateConnectionScan(ctx, input, now)
}

// ConnectionScanByID returns metadata for a worker.
func (s *Store) ConnectionScanByID(ctx context.Context, scanID string) (providerops.ConnectionScan, error) {
	return scanConnectionScan(s.db.QueryRowContext(ctx, connectionScanSelect+` WHERE id=?`, scanID))
}

// ConnectionScanForUser returns an owner-scoped scan without raw result data.
func (s *Store) ConnectionScanForUser(ctx context.Context, scanID, userID string) (providerops.ConnectionScan, error) {
	return scanConnectionScan(s.db.QueryRowContext(ctx, connectionScanSelect+
		` WHERE id=? AND user_id=?`, scanID, userID))
}

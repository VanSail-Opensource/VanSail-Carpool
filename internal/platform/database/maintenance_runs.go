package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// ClaimMaintenanceRun acquires one durable lease for a configured local date.
// Forced claims bypass only the successful-day guard; an unexpired running
// lease still prevents overlap.
func (s *Store) retryAttempt_ClaimMaintenanceRun(ctx context.Context, localDate, leaseOwner string, leaseUntil, now time.Time, force bool) (string, bool, error) {
	localDate, leaseOwner = strings.TrimSpace(localDate), strings.TrimSpace(leaseOwner)
	if _, err := time.Parse(time.DateOnly, localDate); err != nil || leaseOwner == "" || !leaseUntil.After(now) {
		return "", false, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("begin maintenance claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var activeID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM maintenance_runs
		WHERE status='running' AND lease_expires_at>?
		ORDER BY started_at DESC,id DESC LIMIT 1`, stamp(now)).Scan(&activeID)
	if err == nil {
		if commitErr := tx.Commit(); commitErr != nil {
			return "", false, fmt.Errorf("commit held maintenance claim: %w", commitErr)
		}
		return activeID, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("inspect maintenance claim: %w", err)
	}
	if !force {
		var completedID string
		err = tx.QueryRowContext(ctx, `SELECT id FROM maintenance_runs
			WHERE local_date=? AND status='succeeded'
			ORDER BY completed_at DESC,id DESC LIMIT 1`, localDate).Scan(&completedID)
		if err == nil {
			if commitErr := tx.Commit(); commitErr != nil {
				return "", false, fmt.Errorf("commit completed maintenance claim: %w", commitErr)
			}
			return completedID, false, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", false, fmt.Errorf("inspect completed maintenance claim: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE maintenance_runs SET status='failed',error_code='MAINTENANCE_LEASE_EXPIRED',
		lease_expires_at=?,updated_at=?,completed_at=? WHERE status='running' AND lease_expires_at<=?`,
		stamp(now), stamp(now), stamp(now), stamp(now)); err != nil {
		return "", false, fmt.Errorf("expire maintenance claims: %w", err)
	}
	runID, err := ids.New()
	if err != nil {
		return "", false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO maintenance_runs(id,local_date,status,lease_owner,
		lease_expires_at,started_at,updated_at) VALUES(?,?,'running',?,?,?,?)`, runID, localDate,
		leaseOwner, stamp(leaseUntil), stamp(now), stamp(now)); err != nil {
		return "", false, fmt.Errorf("insert maintenance claim: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("commit maintenance claim: %w", err)
	}
	return runID, true, nil
}

// CompleteMaintenanceRun records sanitized outcome and aggregate deletion counts.
func (s *Store) CompleteMaintenanceRun(ctx context.Context, runID, backupRunID string, counts map[string]int64, runErr error, now time.Time) error {
	if counts == nil {
		counts = map[string]int64{}
	}
	for _, count := range counts {
		if count < 0 {
			return ErrConflict
		}
	}
	payload, err := json.Marshal(counts)
	if err != nil {
		return fmt.Errorf("encode maintenance counts: %w", err)
	}
	status, errorCode := "succeeded", ""
	if runErr != nil {
		status, errorCode = "failed", maintenanceErrorCode(runErr)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE maintenance_runs SET status=?,backup_run_id=?,cleanup_counts_json=?,
		error_code=?,lease_expires_at=?,updated_at=?,completed_at=? WHERE id=? AND status='running'`, status,
		nullIfEmpty(backupRunID), string(payload), errorCode, stamp(now), stamp(now), stamp(now), runID)
	if err != nil {
		return fmt.Errorf("complete maintenance run: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		return ErrConflict
	}
	return nil
}

func maintenanceErrorCode(err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "MAINTENANCE_CANCELLED"
	}
	return "MAINTENANCE_FAILED"
}

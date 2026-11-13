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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("begin maintenance claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	activeID, expired, err := maintenanceLeaseCandidatesTx(ctx, tx, now)
	if err != nil {
		return "", false, fmt.Errorf("inspect maintenance claim: %w", err)
	}
	if activeID != "" {
		if commitErr := tx.Commit(); commitErr != nil {
			return "", false, fmt.Errorf("commit held maintenance claim: %w", commitErr)
		}
		return activeID, false, nil
	}
	if !force {
		var completedID string
		err = tx.QueryRowContext(ctx, `SELECT id FROM maintenance_runs
			WHERE local_date=$1 AND status='succeeded'
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
	if len(expired) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE maintenance_runs SET status='failed',error_code='MAINTENANCE_LEASE_EXPIRED',
			lease_expires_at=$1,updated_at=$1,completed_at=$1 WHERE id=ANY($2) AND status='running'`,
			stamp(now), expired); err != nil {
			return "", false, fmt.Errorf("expire maintenance claims: %w", err)
		}
	}
	runID, err := ids.New()
	if err != nil {
		return "", false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO maintenance_runs(id,local_date,status,lease_owner,
		lease_expires_at,started_at,updated_at) VALUES($1,$2,'running',$3,$4,$5,$6)`, runID, localDate,
		leaseOwner, stamp(leaseUntil), stamp(now), stamp(now)); err != nil {
		return "", false, fmt.Errorf("insert maintenance claim: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("commit maintenance claim: %w", err)
	}
	return runID, true, nil
}

// Lock the running set before parsing exact retained lease deadlines. The
// serializable transaction also fences concurrent first claims of an empty set.
func maintenanceLeaseCandidatesTx(ctx context.Context, tx *sql.Tx, now time.Time) (string, []string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,lease_expires_at FROM maintenance_runs
		WHERE status='running' ORDER BY id FOR UPDATE`)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	var active string
	var expired []string
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return "", nil, err
		}
		deadline, err := parseStamp(raw)
		if err != nil {
			return "", nil, err
		}
		if deadline.After(now) {
			if active == "" {
				active = id
			}
		} else {
			expired = append(expired, id)
		}
	}
	return active, expired, rows.Err()
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
	result, err := s.db.ExecContext(ctx, `UPDATE maintenance_runs SET status=$1,backup_run_id=$2,cleanup_counts_json=$3,
		error_code=$4,lease_expires_at=$5,updated_at=$6,completed_at=$7 WHERE id=$8 AND status='running'`, status,
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

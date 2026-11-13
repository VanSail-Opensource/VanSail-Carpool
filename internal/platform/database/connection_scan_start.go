package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// StartConnectionScan records the external-start intent exactly once. Repeated
// queued snapshots cannot both use ordinary progress updates to start a job.
func (s *Store) StartConnectionScan(ctx context.Context, scanID string, now time.Time) (providerops.ConnectionScan, error) {
	started := time.Now()
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (providerops.ConnectionScan, error) {
		scan, err := scanConnectionScan(tx.QueryRowContext(ctx, connectionScanSelect+` WHERE id=$1`, scanID))
		if err != nil {
			return providerops.ConnectionScan{}, err
		}
		// Match cleanup's owner-before-child lock order. The durable start intent
		// must still belong to a live account and an unexpired diagnostics request.
		var accountStatus string
		if err := tx.QueryRowContext(ctx, `SELECT account_status FROM users WHERE id=$1 FOR SHARE`, scan.UserID).Scan(&accountStatus); err != nil {
			return providerops.ConnectionScan{}, err
		}
		scan, err = scanConnectionScan(tx.QueryRowContext(ctx, connectionScanSelect+` WHERE id=$1 FOR UPDATE`, scanID))
		if err != nil {
			return providerops.ConnectionScan{}, err
		}
		if scan.Status != providerops.StatusQueued {
			return providerops.ConnectionScan{}, ErrConflict
		}
		// Account for lock waits and complete transaction retries after the caller
		// captured its clock, while retaining that clock for deterministic callers.
		at := now.UTC().Add(time.Since(started))
		code := ""
		if accountStatus != "active" {
			code = "CONNECTION_SCAN_ACCOUNT_UNAVAILABLE"
		} else if !scan.ExpiresAt.After(at) {
			code = "CONNECTION_SCAN_EXPIRED"
		}
		if code != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE connection_scans SET status='failed',error_code=$1,completed_at=$2,updated_at=$2 WHERE id=$3`, code, stamp(at), scanID); err != nil {
				return providerops.ConnectionScan{}, err
			}
			scan.Status, scan.ErrorCode, scan.UpdatedAt, scan.CompletedAt = providerops.StatusFailed, code, at, &at
			return scan, nil
		}
		if _, err = tx.ExecContext(ctx, `UPDATE connection_scans SET status='processing',error_code='',updated_at=$1 WHERE id=$2`, stamp(at), scanID); err != nil {
			return providerops.ConnectionScan{}, err
		}
		scan.Status, scan.ErrorCode, scan.UpdatedAt = providerops.StatusProcessing, "", at
		return scan, nil
	})
}

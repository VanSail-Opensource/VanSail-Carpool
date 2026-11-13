package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// StartConnectionScan records the external-start intent exactly once. Repeated
// queued snapshots cannot both use ordinary progress updates to start a job.
func (s *Store) retryAttempt_StartConnectionScan(ctx context.Context, scanID string, now time.Time) (providerops.ConnectionScan, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.ConnectionScan{}, err
	}
	defer tx.Rollback()
	scan, err := scanConnectionScan(tx.QueryRowContext(ctx, connectionScanSelect+` WHERE id=$1 FOR UPDATE`, scanID))
	if err != nil {
		return providerops.ConnectionScan{}, err
	}
	if scan.Status != providerops.StatusQueued {
		return providerops.ConnectionScan{}, ErrConflict
	}
	now = now.UTC()
	if _, err = tx.ExecContext(ctx, `UPDATE connection_scans SET status='processing',error_code='',updated_at=$1 WHERE id=$2`, stamp(now), scanID); err != nil {
		return providerops.ConnectionScan{}, err
	}
	scan.Status, scan.ErrorCode, scan.UpdatedAt = providerops.StatusProcessing, "", now
	if err = tx.Commit(); err != nil {
		return providerops.ConnectionScan{}, err
	}
	return scan, nil
}

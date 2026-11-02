package database

import (
	"context"
	"fmt"
	"time"
)

// RecoverConnectivityAttempts interrupts probes whose process exited before completion.
// The original start time remains authoritative for retention and ordering.
func (s *Store) RecoverConnectivityAttempts(ctx context.Context, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	at := connectivityStamp(now)
	_, err := s.db.ExecContext(ctx, `UPDATE host_connectivity_attempts SET status='interrupted',
		finished_at=CASE WHEN started_at>$1 THEN started_at ELSE $2 END,error_code='CONNECTIVITY_INTERRUPTED'
		WHERE status='running'`, at, at)
	if err != nil {
		return fmt.Errorf("recover connectivity attempts: %w", err)
	}
	return nil
}

// PruneConnectivityAttempts expires attempts exactly 24 hours after their start.
// Callers run this independently of the scheduled-check enablement setting.
func (s *Store) PruneConnectivityAttempts(ctx context.Context, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM host_connectivity_attempts WHERE started_at<=$1`,
		connectivityStamp(now.Add(-connectivityRetention)))
	if err != nil {
		return fmt.Errorf("prune connectivity attempts: %w", err)
	}
	return nil
}

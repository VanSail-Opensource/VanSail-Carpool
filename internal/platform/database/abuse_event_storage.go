package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func (s *Store) StoreEvents(ctx context.Context, nodeID string, events []abuse.LogEvent, counts abuse.ReportCounts, now time.Time) (abuse.ReportCounts, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, err
	}
	defer func() { _ = tx.Rollback() }()
	for _, event := range events {
		result, execErr := tx.ExecContext(ctx, `INSERT OR IGNORE INTO abuse_log_fingerprints(node_uuid,fingerprint,observed_at) VALUES(?,?,?)`, nodeID, event.Fingerprint, stamp(now))
		if execErr != nil {
			return counts, execErr
		}
		changed, _ := result.RowsAffected()
		if changed == 0 {
			counts.Duplicate++
			continue
		}
		result, execErr = tx.ExecContext(ctx, `INSERT OR IGNORE INTO abuse_pending_log_events(user_id,node_uuid,event_second,domain,fingerprint,received_at) VALUES(?,?,?,?,?,?)`, event.UserID, nodeID, stamp(event.EventSecond), event.Domain, event.Fingerprint, stamp(now))
		if execErr != nil {
			return counts, fmt.Errorf("store normalized abuse event: %w", execErr)
		}
		changed, _ = result.RowsAffected()
		if changed == 0 {
			counts.Duplicate++
			continue
		}
		counts.Accepted++
	}
	if err = tx.Commit(); err != nil {
		return counts, err
	}
	return counts, nil
}

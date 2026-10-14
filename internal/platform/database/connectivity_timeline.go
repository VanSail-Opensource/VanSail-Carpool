package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

// ConnectivityTimeline reads retained terminal observations for one configuration.
// Setup failures have no host and cannot claim an outage. No pagination truncates uptime.
func (s *Store) ConnectivityTimeline(ctx context.Context, configHash string, now time.Time) ([]connectivity.Attempt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+connectivityAttemptColumns+` FROM host_connectivity_attempts
		WHERE config_hash=? AND host_uuid IS NOT NULL AND started_at>? AND finished_at<=?
		ORDER BY host_uuid,finished_at,started_at,id`, configHash, connectivityStamp(now.Add(-connectivityRetention)), connectivityStamp(now))
	if err != nil {
		return nil, fmt.Errorf("read connectivity timeline: %w", err)
	}
	return collectConnectivityAttempts(rows, 0)
}

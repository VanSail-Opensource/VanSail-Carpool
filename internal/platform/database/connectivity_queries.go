package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/google/uuid"
)

// LatestConnectivityAttempts derives one retained latest attempt per host for a configuration.
func (s *Store) LatestConnectivityAttempts(ctx context.Context, configHash string, now time.Time) ([]connectivity.Attempt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+connectivityAttemptColumns+` FROM (
		SELECT `+connectivityAttemptColumns+`,ROW_NUMBER() OVER
		(PARTITION BY host_uuid ORDER BY started_at DESC,id DESC) AS freshness
		FROM host_connectivity_attempts WHERE config_hash=? AND host_uuid IS NOT NULL AND started_at>?
		) WHERE freshness=1 ORDER BY host_uuid`, configHash, connectivityStamp(now.Add(-connectivityRetention)))
	if err != nil {
		return nil, fmt.Errorf("query latest connectivity attempts: %w", err)
	}
	return collectConnectivityAttempts(rows, 0)
}

// ConnectivityHistory returns a stable descending page strictly within the rolling retention window.
func (s *Store) ConnectivityHistory(ctx context.Context, hostUUID, cursor string, limit int, now time.Time) (connectivity.HistoryPage, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	hostUUID = strings.ToLower(strings.TrimSpace(hostUUID))
	if hostUUID != "" && (!connectivityHostPattern.MatchString(hostUUID) || hostUUID == uuid.Nil.String()) {
		return connectivity.HistoryPage{}, ErrConflict
	}
	query := `SELECT ` + connectivityAttemptColumns + ` FROM host_connectivity_attempts WHERE started_at>?`
	args := []any{connectivityStamp(now.Add(-connectivityRetention))}
	if hostUUID != "" {
		query += ` AND host_uuid=?`
		args = append(args, hostUUID)
	}
	filter := pageFilterFingerprint("connectivity", hostUUID)
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return connectivity.HistoryPage{}, err
		}
		at, err := parseStamp(decoded.Timestamp)
		if err != nil {
			return connectivity.HistoryPage{}, ErrInvalidCursor
		}
		cursorAt := connectivityStamp(at)
		query += ` AND (started_at<? OR (started_at=? AND id<?))`
		args = append(args, cursorAt, cursorAt, decoded.ID)
	}
	query += ` ORDER BY started_at DESC,id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return connectivity.HistoryPage{}, fmt.Errorf("query connectivity history: %w", err)
	}
	items, err := collectConnectivityAttempts(rows, limit+1)
	if err != nil {
		return connectivity.HistoryPage{}, err
	}
	page := connectivity.HistoryPage{Items: items}
	if len(items) <= limit {
		return page, nil
	}
	page.Items = items[:limit]
	last := page.Items[limit-1]
	next, err := encodeTimestampCursor(last.StartedAt, last.ID, filter)
	if err != nil {
		return connectivity.HistoryPage{}, err
	}
	page.NextCursor = &next
	return page, nil
}

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Preserve live bearers using parsed instants, including timestamps that differ
// by less than PostgreSQL's microsecond. The schema already permits only one
// Mini App session per member; browser sessions remain independent.
func pruneAuthTokensTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	for _, item := range []struct{ key, selectQuery, deleteQuery string }{
		{"sessions", `SELECT encode(token_hash,'hex'),expires_at FROM sessions WHERE expires_at<$1`,
			`DELETE FROM sessions WHERE token_hash IN (SELECT decode(value,'hex') FROM unnest($1::text[]) AS keys(value))`},
		{"browser_access_links", `SELECT encode(token_hash,'hex'),expires_at FROM browser_access_links WHERE expires_at<$1`,
			`DELETE FROM browser_access_links WHERE token_hash IN (SELECT decode(value,'hex') FROM unnest($1::text[]) AS keys(value))`},
	} {
		counts[item.key] = 0
		expired, err := expiredTimestampIDsTx(ctx, tx, item.selectQuery, now)
		if err != nil {
			return fmt.Errorf("find expired %s: %w", item.key, err)
		}
		count, err := pruneTimestampIDsTx(ctx, tx, item.deleteQuery, expired)
		if err != nil {
			return fmt.Errorf("prune expired %s: %w", item.key, err)
		}
		counts[item.key] += count
	}
	return nil
}

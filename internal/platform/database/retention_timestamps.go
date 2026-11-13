package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RFC3339Nano has variable fractional precision, so lexical comparison alone
// can expire a record just after a whole-second boundary. Use an indexed broad
// candidate bound, then compare parsed UTC instants at full precision.
func expiredTimestampIDsTx(ctx context.Context, tx *sql.Tx, query string, cutoff time.Time, args ...any) ([]string, error) {
	params := append([]any{retentionCandidateUpperBound(cutoff)}, args...)
	rows, err := tx.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := make([]string, 0)
	for rows.Next() {
		var id, value string
		if err := rows.Scan(&id, &value); err != nil {
			return nil, err
		}
		at, err := parseStamp(value)
		if err != nil {
			return nil, fmt.Errorf("parse retention timestamp: %w", err)
		}
		if !at.After(cutoff) {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// The index predicate admits the whole cutoff second; exact comparisons decide
// which candidates may be retired without rewriting retained timestamps.
func retentionCandidateUpperBound(cutoff time.Time) string {
	return stamp(cutoff.UTC().Truncate(time.Second).Add(time.Second))
}

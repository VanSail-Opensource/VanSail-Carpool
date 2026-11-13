package database

import (
	"context"
	"database/sql"
	"time"
)

// XNodeCompileVersion reads one small revision row, independent of traffic
// report writes. The next purchase boundary invalidates time-derived access.
func (s *Store) XNodeCompileVersion(ctx context.Context, now time.Time) (int64, time.Time, error) {
	var revision int64
	var purchaseBoundary sql.NullString
	var hourlyBoundary sql.NullTime
	// Stored terms use stamp's UTC format. Widen the indexed text candidate
	// range, then normalize fractional precision without rounding to PostgreSQL's
	// microseconds. Retained stamps may contain equivalent trailing zeroes.
	lower := stamp(now.UTC().Truncate(time.Second).Add(-time.Second))
	err := s.db.QueryRowContext(ctx, `SELECT revision,(SELECT MIN(boundary) FROM (
	 SELECT MIN(`+purchaseTimestampOrderSQL("valid_from")+`) AS boundary FROM purchases WHERE valid_from>$2 AND `+purchaseTimestampOrderSQL("valid_from")+`>$1
	 UNION ALL SELECT MIN(`+purchaseTimestampOrderSQL("valid_until")+`) FROM purchases WHERE valid_until>$2 AND `+purchaseTimestampOrderSQL("valid_until")+`>$1
	)),(SELECT MIN(paid_through) FROM hourly_enrollments WHERE status='active' AND paid_through>$3)
	 FROM xnode_compile_revision WHERE singleton=1`, purchaseTimestampOrder(now), lower, now.UTC().Truncate(time.Microsecond)).Scan(&revision, &purchaseBoundary, &hourlyBoundary)
	if err != nil {
		return revision, time.Time{}, err
	}
	var boundary time.Time
	if purchaseBoundary.Valid {
		boundary, err = parseStamp(purchaseBoundary.String + "Z")
		if err != nil {
			return revision, time.Time{}, err
		}
	}
	if hourlyBoundary.Valid && (boundary.IsZero() || hourlyBoundary.Time.Before(boundary)) {
		boundary = hourlyBoundary.Time.UTC()
	}
	return revision, boundary, nil
}

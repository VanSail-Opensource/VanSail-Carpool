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
	var boundary sql.NullTime
	// Stored terms use stamp's UTC format. Widen the indexed text candidate
	// range, then compare instants so fractional seconds cannot hide a boundary.
	lower := stamp(now.UTC().Truncate(time.Second).Add(-time.Second))
	err := s.db.QueryRowContext(ctx, `SELECT revision,(SELECT MIN(boundary) FROM (
	 SELECT MIN(valid_from::timestamptz) AS boundary FROM purchases WHERE valid_from>$3 AND valid_from::timestamptz>($1)::timestamptz
	 UNION ALL SELECT MIN(valid_until::timestamptz) FROM purchases WHERE valid_until>$3 AND valid_until::timestamptz>($2)::timestamptz
	 UNION ALL SELECT MIN(paid_through) FROM hourly_enrollments WHERE status='active' AND paid_through>($2)::timestamptz
	)) FROM xnode_compile_revision WHERE singleton=1`, stamp(now), stamp(now), lower).Scan(&revision, &boundary)
	if err != nil || !boundary.Valid {
		return revision, time.Time{}, err
	}
	return revision, boundary.Time.UTC(), nil
}

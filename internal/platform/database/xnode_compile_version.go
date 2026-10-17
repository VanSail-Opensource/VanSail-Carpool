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
	var boundary sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT revision,(SELECT MIN(boundary) FROM (
	 SELECT MIN(valid_from) AS boundary FROM purchases WHERE valid_from>?
	 UNION ALL SELECT MIN(valid_until) FROM purchases WHERE valid_until>?
	)) FROM xnode_compile_revision WHERE singleton=1`, stamp(now), stamp(now)).Scan(&revision, &boundary)
	if err != nil || !boundary.Valid {
		return revision, time.Time{}, err
	}
	next, err := parseStamp(boundary.String)
	return revision, next, err
}

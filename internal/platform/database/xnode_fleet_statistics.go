package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

// Fleet accounting excludes all connection, session and live-rate reports.
func (s *Store) XNodeFleetUsage(ctx context.Context, from, to time.Time) ([]xnode.DailyUsage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,date,upload_bytes,download_bytes,weighted_bytes FROM xnode_daily_usage WHERE date>=? AND date<=? UNION ALL SELECT node_id,date,raw_bytes,0,weighted_bytes FROM xnode_legacy_history WHERE date>=? AND date<=?`, from.UTC().Format(time.DateOnly), to.UTC().Format(time.DateOnly), from.UTC().Format(time.DateOnly), to.UTC().Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.DailyUsage{}
	for rows.Next() {
		var item xnode.DailyUsage
		if err = rows.Scan(&item.NodeID, &item.Date, &item.UploadBytes, &item.DownloadBytes, &item.WeightedBytes); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) XNodeIdentityGrowth(ctx context.Context, from, to time.Time) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_identities WHERE user_id IS NOT NULL AND created_at>=? AND created_at<=?`, stamp(from), stamp(to)).Scan(&count)
	return count, err
}

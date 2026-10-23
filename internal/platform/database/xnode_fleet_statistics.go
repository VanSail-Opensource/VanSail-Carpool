package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"math"
	"time"
)

// Fleet accounting excludes all connection, session and live-rate reports.
func (s *Store) XNodeFleetUsage(ctx context.Context, from, to time.Time) ([]xnode.DailyUsage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,date,upload_bytes,download_bytes,weighted_bytes FROM xnode_daily_usage WHERE date>=? AND date<=? UNION ALL SELECT node_id,date,raw_bytes,0,weighted_bytes FROM xnode_imported_history WHERE date>=? AND date<=? ORDER BY node_id,date`, from.UTC().Format(time.DateOnly), to.UTC().Format(time.DateOnly), from.UTC().Format(time.DateOnly), to.UTC().Format(time.DateOnly))
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
		if item.UploadBytes < 0 || item.DownloadBytes < 0 || item.WeightedBytes < 0 {
			return nil, xnode.ErrInvalid
		}
		if len(out) > 0 && out[len(out)-1].NodeID == item.NodeID && out[len(out)-1].Date == item.Date {
			previous := &out[len(out)-1]
			if previous.UploadBytes > math.MaxInt64-item.UploadBytes || previous.DownloadBytes > math.MaxInt64-item.DownloadBytes || previous.WeightedBytes > math.MaxInt64-item.WeightedBytes {
				return nil, xnode.ErrInvalid
			}
			previous.UploadBytes += item.UploadBytes
			previous.DownloadBytes += item.DownloadBytes
			previous.WeightedBytes += item.WeightedBytes
		} else {
			out = append(out, item)
		}
	}
	return out, rows.Err()
}
func (s *Store) XNodeIdentityGrowth(ctx context.Context, from, to time.Time) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_identities WHERE user_id IS NOT NULL AND created_at>=? AND created_at<=?`, stamp(from), stamp(to)).Scan(&count)
	return count, err
}

package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

func (s *Store) XNodeImportedHistory(ctx context.Context, user int64, from, to time.Time) ([]xnode.ImportedHistory, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT proxy_user_id,node_id,date,raw_bytes,weighted_bytes FROM xnode_imported_history WHERE proxy_user_id=? AND date>=? AND date<=? ORDER BY date,node_id`, user, from.UTC().Format(time.DateOnly), to.UTC().Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []xnode.ImportedHistory{}
	for rows.Next() {
		var value xnode.ImportedHistory
		if err = rows.Scan(&value.ProxyUserID, &value.NodeID, &value.Date, &value.RawBytes, &value.WeightedBytes); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

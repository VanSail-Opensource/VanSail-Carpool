package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

// SaveXNodeLegacyHistory is an idempotent cumulative export. It does not change
// period usage: the source aggregate is already weighted and synced separately.
func (s *Store) SaveXNodeLegacyHistory(ctx context.Context, values []xnode.LegacyHistory) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, value := range values {
		if value.ProxyUserID <= 0 || value.NodeID == "" || value.RawBytes < 0 || value.WeightedBytes < 0 {
			return xnode.ErrInvalid
		}
		if _, err := time.Parse(time.DateOnly, value.Date); err != nil {
			return xnode.ErrInvalid
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO xnode_legacy_history(proxy_user_id,node_id,date,raw_bytes,weighted_bytes) VALUES(?,?,?,?,?) ON CONFLICT(proxy_user_id,node_id,date) DO UPDATE SET raw_bytes=excluded.raw_bytes,weighted_bytes=excluded.weighted_bytes WHERE xnode_legacy_history.raw_bytes<=excluded.raw_bytes AND xnode_legacy_history.weighted_bytes<=excluded.weighted_bytes`, value.ProxyUserID, value.NodeID, value.Date, value.RawBytes, value.WeightedBytes)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return xnode.ErrConflict
		}
	}
	return tx.Commit()
}
func (s *Store) XNodeLegacyHistory(ctx context.Context, user int64, from, to time.Time) ([]xnode.LegacyHistory, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT proxy_user_id,node_id,date,raw_bytes,weighted_bytes FROM xnode_legacy_history WHERE proxy_user_id=? AND date>=? AND date<=? ORDER BY date,node_id`, user, from.UTC().Format(time.DateOnly), to.UTC().Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []xnode.LegacyHistory{}
	for rows.Next() {
		var value xnode.LegacyHistory
		if err = rows.Scan(&value.ProxyUserID, &value.NodeID, &value.Date, &value.RawBytes, &value.WeightedBytes); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"

	"time"
)

func (s *Store) XNodeImportedUsage(ctx context.Context, id int64) (xnode.ImportedUsage, error) {
	var result xnode.ImportedUsage
	var reset sql.NullString
	var at string
	err := s.db.QueryRowContext(ctx, `SELECT proxy_user_id,COALESCE(period_id,''),used_bytes,lifetime_bytes,last_reset_at,synced_at FROM xnode_imported_usage WHERE proxy_user_id=?`, id).Scan(&result.ProxyUserID, &result.PeriodID, &result.UsedBytes, &result.LifetimeBytes, &reset, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return result, xnode.ErrNotFound
	}
	if err != nil {
		return result, err
	}
	result.SyncedAt, err = time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return result, err
	}
	if reset.Valid {
		value, err := time.Parse(time.RFC3339Nano, reset.String)
		if err != nil {
			return result, err
		}
		result.LastResetAt = &value
	}
	return result, nil
}

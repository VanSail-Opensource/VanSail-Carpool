package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"math"
	"time"
)

func (s *Store) XNodeLegacyUsage(ctx context.Context, id int64) (xnode.LegacyUsage, error) {
	var result xnode.LegacyUsage
	var reset sql.NullString
	var at string
	err := s.db.QueryRowContext(ctx, `SELECT proxy_user_id,COALESCE(period_id,''),used_bytes,lifetime_bytes,last_reset_at,synced_at FROM xnode_legacy_usage WHERE proxy_user_id=?`, id).Scan(&result.ProxyUserID, &result.PeriodID, &result.UsedBytes, &result.LifetimeBytes, &reset, &at)
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

// SyncXNodeLegacyUsage imports the already weighted legacy aggregate exactly
// once per snapshot. Native bytes stay in the same counter without double pricing.
func (s *Store) SyncXNodeLegacyUsage(ctx context.Context, next xnode.LegacyUsage) error {
	if next.UsedBytes < 0 || next.LifetimeBytes < next.UsedBytes || next.SyncedAt.IsZero() {
		return xnode.ErrInvalid
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previousBytes, previousLifetime int64
	var previousPeriod, previousAt string
	var previousReset sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT used_bytes,lifetime_bytes,COALESCE(period_id,''),synced_at,last_reset_at FROM xnode_legacy_usage WHERE proxy_user_id=?`, next.ProxyUserID).Scan(&previousBytes, &previousLifetime, &previousPeriod, &previousAt, &previousReset)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	hasPrevious := err == nil
	if hasPrevious {
		at, err := time.Parse(time.RFC3339Nano, previousAt)
		if err != nil {
			return err
		}
		if !next.SyncedAt.After(at) {
			return tx.Commit()
		}
		if next.LifetimeBytes < previousLifetime {
			return xnode.ErrConflict
		}
	}
	var reset any
	resetText := ""
	if next.LastResetAt != nil {
		resetText = stamp(*next.LastResetAt)
		reset = resetText
	}
	delta := next.UsedBytes
	if hasPrevious && previousReset.String == resetText {
		if next.UsedBytes < previousBytes {
			return xnode.ErrConflict
		}
		delta -= previousBytes
		// Changing native purchase IDs cannot reset an upstream cumulative counter.
		// Until the source reports its reset, its last open term owns these bytes.
		if previousPeriod != "" {
			next.PeriodID = previousPeriod
		}
	} else if hasPrevious && previousPeriod != "" {
		// Lifetime survives an upstream reset. Recover the final old-period tail
		// even when it arrived after the panel opened the new paid period.
		tail := next.LifetimeBytes - previousLifetime - next.UsedBytes
		if tail < 0 {
			return xnode.ErrConflict
		}
		if tail > 0 {
			if err := addLegacyPeriodBytes(ctx, tx, previousPeriod, next.ProxyUserID, tail); err != nil {
				return err
			}
		}
	}
	if next.PeriodID != "" {
		if err := addLegacyPeriodBytes(ctx, tx, next.PeriodID, next.ProxyUserID, delta); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_legacy_usage(proxy_user_id,period_id,used_bytes,lifetime_bytes,last_reset_at,synced_at) VALUES(?,?,?,?,?,?) ON CONFLICT(proxy_user_id) DO UPDATE SET period_id=excluded.period_id,used_bytes=excluded.used_bytes,lifetime_bytes=excluded.lifetime_bytes,last_reset_at=excluded.last_reset_at,synced_at=excluded.synced_at`, next.ProxyUserID, nullText(next.PeriodID), next.UsedBytes, next.LifetimeBytes, reset, stamp(next.SyncedAt))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func addLegacyPeriodBytes(ctx context.Context, tx *sql.Tx, period string, user, delta int64) error {
	result, err := tx.ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=used_bytes+? WHERE id=? AND proxy_user_id=? AND status IN('open','closing') AND used_bytes<=?`, delta, period, user, math.MaxInt64-delta)
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
	return nil
}

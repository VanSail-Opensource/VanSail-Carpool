package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/usdt"
)

// AdmitUSDTRPC consumes a durable quota before any network traffic. Minute
// buckets conservatively include the boundary bucket in rolling windows.
func (s *Store) AdmitUSDTRPC(ctx context.Context, g usdt.QuotaGroup, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var nextRaw, cooldownRaw string
	err = tx.QueryRowContext(ctx, "SELECT next_at,cooldown_until FROM usdt_rpc_state WHERE group_id=?", g.ID).Scan(&nextRaw, &cooldownRaw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	for _, raw := range []string{nextRaw, cooldownRaw} {
		if raw != "" {
			until, err := parseStamp(raw)
			if err != nil {
				return err
			}
			if now.Before(until) {
				return &usdt.Pause{Until: until, Code: "USDT_RPC_COOLDOWN"}
			}
		}
	}
	minuteStart := now.Add(-time.Minute).Truncate(time.Minute)
	dayStart := now.Add(-24 * time.Hour).Truncate(time.Minute)
	var minuteUsed, dayUsed int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN minute>=? THEN count ELSE 0 END),0),COALESCE(SUM(count),0) FROM usdt_rpc_usage WHERE group_id=? AND minute>=?`, stamp(minuteStart), g.ID, stamp(dayStart)).Scan(&minuteUsed, &dayUsed); err != nil {
		return err
	}
	if minuteUsed >= g.RequestsPerMinute || dayUsed >= g.RequestsPerDay {
		start := minuteStart
		window := time.Minute
		if dayUsed >= g.RequestsPerDay {
			start = dayStart
			window = 24 * time.Hour
		}
		var oldest string
		if err := tx.QueryRowContext(ctx, "SELECT MIN(minute) FROM usdt_rpc_usage WHERE group_id=? AND minute>=?", g.ID, stamp(start)).Scan(&oldest); err != nil {
			return err
		}
		when, err := parseStamp(oldest)
		if err != nil {
			return err
		}
		return &usdt.Pause{Until: when.Add(window + time.Minute), Code: "USDT_RPC_QUOTA_EXHAUSTED"}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO usdt_rpc_usage(group_id,minute,count) VALUES(?,?,1) ON CONFLICT(group_id,minute) DO UPDATE SET count=count+1`, g.ID, stamp(now.Truncate(time.Minute))); err != nil {
		return err
	}
	next := now.Add(time.Second / time.Duration(g.RequestsPerSecond))
	if _, err := tx.ExecContext(ctx, `INSERT INTO usdt_rpc_state(group_id,next_at,cooldown_until,failures) VALUES(?,?,?,0) ON CONFLICT(group_id) DO UPDATE SET next_at=excluded.next_at`, g.ID, stamp(next), stamp(time.Time{})); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM usdt_rpc_usage WHERE minute<?", stamp(dayStart)); err != nil {
		return err
	}
	return tx.Commit()
}

// FinishUSDTRPC retains backoff across restarts; a successful call resets failures.
func (s *Store) FinishUSDTRPC(ctx context.Context, group string, until time.Time, success bool) error {
	if success {
		_, err := s.db.ExecContext(ctx, "UPDATE usdt_rpc_state SET failures=0 WHERE group_id=?", group)
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var failures int
	var oldRaw string
	if err := tx.QueryRowContext(ctx, "SELECT failures,cooldown_until FROM usdt_rpc_state WHERE group_id=?", group).Scan(&failures, &oldRaw); err != nil {
		return err
	}
	backoff := time.Second * time.Duration(1<<min(failures, 8))
	minimum := time.Now().UTC().Add(backoff)
	if until.Before(minimum) {
		until = minimum
	}
	old, err := parseStamp(oldRaw)
	if err != nil {
		return err
	}
	if old.After(until) {
		until = old
	}
	if _, err := tx.ExecContext(ctx, "UPDATE usdt_rpc_state SET failures=failures+1,cooldown_until=? WHERE group_id=?", stamp(until), group); err != nil {
		return err
	}
	return tx.Commit()
}

// USDTQuotaStatus is local-only; displaying status never probes an upstream.
func (s *Store) USDTQuotaStatus(ctx context.Context, groups []usdt.QuotaGroup, now time.Time) ([]usdt.QuotaStatus, error) {
	result := []usdt.QuotaStatus{}
	for _, g := range groups {
		q := usdt.QuotaStatus{ID: g.ID}
		var until sql.NullString
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN minute>=? THEN count ELSE 0 END),0),COALESCE(SUM(count),0) FROM usdt_rpc_usage WHERE group_id=? AND minute>=?`, stamp(now.Add(-time.Minute).Truncate(time.Minute)), g.ID, stamp(now.Add(-24*time.Hour).Truncate(time.Minute))).Scan(&q.MinuteUsed, &q.DayUsed); err != nil {
			return nil, err
		}
		err := s.db.QueryRowContext(ctx, "SELECT cooldown_until FROM usdt_rpc_state WHERE group_id=?", g.ID).Scan(&until)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if until.Valid {
			t, err := parseStamp(until.String)
			if err != nil {
				return nil, err
			}
			if t.After(now) {
				q.CooldownUntil = &t
			}
		}
		result = append(result, q)
	}
	return result, nil
}

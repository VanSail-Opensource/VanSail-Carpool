package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodePeriod(ctx context.Context, id string) (xnode.Period, error) {
	return scanXNodePeriod(s.db.QueryRowContext(ctx, `SELECT `+xnodePeriodColumns+` FROM xnode_periods WHERE id=?`, id))
}

const xnodePeriodColumns = "id,proxy_user_id,COALESCE(purchase_id,''),started_at,ended_at,limit_bytes,strategy,last_reset_at,used_bytes,status"

func scanXNodePeriod(row rowScanner) (xnode.Period, error) {
	var item xnode.Period
	var started string
	var ended, reset sql.NullString
	err := row.Scan(&item.ID, &item.ProxyUserID, &item.PurchaseID, &started, &ended, &item.LimitBytes, &item.Strategy, &reset, &item.UsedBytes, &item.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return item, xnode.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	item.StartedAt, err = time.Parse(time.RFC3339Nano, started)
	if err != nil {
		return item, err
	}
	if ended.Valid {
		value, e := time.Parse(time.RFC3339Nano, ended.String)
		if e != nil {
			return item, e
		}
		item.EndedAt = &value
	}
	if reset.Valid {
		value, e := time.Parse(time.RFC3339Nano, reset.String)
		if e != nil {
			return item, e
		}
		item.LastResetAt = &value
	}
	return item, nil
}
func (s *Store) retryAttempt_SaveXNodePeriod(ctx context.Context, item xnode.Period) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var ended, reset any
	if item.EndedAt != nil {
		ended = stamp(*item.EndedAt)
	}
	if item.LastResetAt != nil {
		reset = stamp(*item.LastResetAt)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,purchase_id,started_at,ended_at,limit_bytes,strategy,last_reset_at,status) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET ended_at=excluded.ended_at,limit_bytes=excluded.limit_bytes,strategy=excluded.strategy,last_reset_at=excluded.last_reset_at,status=excluded.status WHERE xnode_periods.proxy_user_id=excluded.proxy_user_id AND xnode_periods.purchase_id IS excluded.purchase_id AND NOT (xnode_periods.status IN ('closing','closed','review') AND excluded.status='open')`, item.ID, item.ProxyUserID, nullText(item.PurchaseID), stamp(item.StartedAt), ended, item.LimitBytes, item.Strategy, reset, item.Status)
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
	if item.Status == "open" {
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=? WHERE proxy_user_id=?`, item.ID, item.ProxyUserID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) XNodeUsage(ctx context.Context, id int64, from, to time.Time) ([]xnode.DailyUsage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes,weighted_bytes FROM xnode_daily_usage WHERE proxy_user_id=? AND date>=? AND date<=? ORDER BY date,node_id,period_id`, id, from.UTC().Format(time.DateOnly), to.UTC().Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.DailyUsage{}
	for rows.Next() {
		var item xnode.DailyUsage
		if err = rows.Scan(&item.ProxyUserID, &item.NodeID, &item.PeriodID, &item.Date, &item.UploadBytes, &item.DownloadBytes, &item.WeightedBytes); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) XNodeReportSequence(ctx context.Context, agent string) (uint64, error) {
	var sequence uint64
	err := s.db.QueryRowContext(ctx, `SELECT last_sequence FROM xnode_report_cursors WHERE agent_id=?`, agent).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return sequence, err
}

func (s *Store) XNodeClosingPeriods(ctx context.Context, limit int) ([]xnode.Period, error) {
	if limit < 1 || limit > 128 {
		return nil, xnode.ErrInvalid
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM xnode_periods WHERE status='closing' ORDER BY ended_at,id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil || closeErr != nil {
		return nil, errors.Join(err, closeErr)
	}
	result := []xnode.Period{}
	for _, id := range ids {
		item, err := s.XNodePeriod(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

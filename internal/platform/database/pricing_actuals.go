package database

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (s *Store) pricingActual(ctx context.Context, kind, id, metric string, from, until time.Time) (*float64, string, error) {
	var value sql.NullFloat64
	var node string
	financial := false
	args := []any{id, from.Format("2006-01-02"), until.Format("2006-01-02")}
	query := `SELECT SUM(upload_bytes+download_bytes) FROM xnode_daily_usage WHERE node_id=? AND date>=? AND date<?`
	switch kind {
	case "user":
		return s.pricingUserActual(ctx, id, from, until)
	case "node":
		node = id
	case "user_node":
		parts := strings.SplitN(id, ":", 2)
		if len(parts) != 2 {
			return nil, "", ErrConflict
		}
		node = parts[1]
		args = []any{parts[0], node, from.Format("2006-01-02"), until.Format("2006-01-02")}
		query = `SELECT SUM(d.upload_bytes+d.download_bytes) FROM xnode_daily_usage d JOIN xnode_identities i ON i.proxy_user_id=d.proxy_user_id LEFT JOIN xnode_payg_accounts a ON a.proxy_user_id=d.proxy_user_id WHERE COALESCE(i.user_id,a.user_id)=? AND d.node_id=? AND d.date>=? AND d.date<?`
	case "product":
		if metric != "gross_revenue_minor" {
			return nil, "", ErrConflict
		}
		parts := strings.SplitN(id, ":", 2)
		if len(parts) != 2 {
			return nil, "", ErrConflict
		}
		financial = true
		args = []any{parts[1], pricingStamp(from), pricingStamp(until)}
		query = `SELECT COALESCE(-SUM(l.delta_txb_minor),0)` + pricingComboReceiptJoins + ` WHERE ` + pricingComboReceiptScope + ` AND l.created_at>=? AND l.created_at<?`
		if parts[0] == "payg" {
			query = `SELECT COALESCE(SUM(c.blocks*c.unit_price_minor),0) FROM xnode_payg_charges c JOIN xnode_payg_accounts a ON a.proxy_user_id=c.proxy_user_id JOIN ledger_entries l ON l.id=c.ledger_id WHERE a.group_id=? AND l.created_at>=? AND l.created_at<?`
		}
	default:
		return nil, "", ErrConflict
	}
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&value); err != nil {
		return nil, "", err
	}
	if !financial {
		var incidents int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_operational_incidents WHERE (scope_type='global' OR scope_type='node' AND scope_id=?) AND started_at<? AND (ended_at IS NULL OR ended_at>?)`, node, pricingStamp(until), pricingStamp(from)).Scan(&incidents); err != nil {
			return nil, "", err
		}
		if incidents > 0 {
			return nil, "CENSORED", nil
		}
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_admin_overrides WHERE command='SUPPRESS_TELEMETRY' AND status IN ('ACTIVE','EXPIRED') AND (scope_type='global' OR scope_type IN ('node','machine') AND scope_id=?) AND starts_at<? AND expires_at>?`, node, pricingStamp(until), pricingStamp(from)).Scan(&incidents); err != nil {
			return nil, "", err
		}
		if incidents > 0 {
			return nil, "CENSORED", nil
		}
	}
	if !value.Valid {
		return nil, "MISSING", nil
	}
	return &value.Float64, "COMPLETE", nil
}

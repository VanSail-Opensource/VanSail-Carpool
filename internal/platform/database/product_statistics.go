package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ProductDatabaseStatistics calculates range-free metrics from durable facts.
func (s *Store) ProductDatabaseStatistics(ctx context.Context, now time.Time) (model.DatabaseStatistics, error) {
	result := model.DatabaseStatistics{}
	totalUsers, err := integerQuery(ctx, s.db, `SELECT COUNT(*) FROM users WHERE role='user'`)
	if err != nil {
		return result, err
	}
	buyers, err := integerQuery(ctx, s.db, `SELECT COUNT(*) FROM users member WHERE member.role='user' AND (
		EXISTS (SELECT 1 FROM purchases purchase WHERE purchase.user_id=member.id)
		OR EXISTS (SELECT 1 FROM purchase_history_tombstones purchase WHERE purchase.user_id=member.id))`)
	if err != nil {
		return result, err
	}
	if totalUsers > 0 {
		result.NewUserConversion = float64(buyers) * 100 / float64(totalUsers)
	}
	average, minimum, maximum, err := s.spendStatistics(ctx)
	if err != nil {
		return result, err
	}
	result.AverageSpend, result.SpendMinimum, result.SpendMaximum = model.TXBMoney(average), model.TXBMoney(minimum), model.TXBMoney(maximum)
	if result.SubscriptionStates, err = s.subscriptionStateShares(ctx, now); err != nil {
		return result, err
	}
	checkIn, err := aggregateAverageMinor(ctx, s.db, `SELECT COALESCE(SUM(total_minor),0),COALESCE(SUM(fact_count),0) FROM (
		SELECT COALESCE(SUM(reward_minor),0) total_minor,COUNT(*) fact_count FROM activity_daily_checkins
		UNION ALL
		SELECT COALESCE(SUM(checkin_reward_txb_minor),0),COALESCE(SUM(checkin_count),0) FROM activity_daily_rollups)`)
	if err != nil {
		return result, err
	}
	result.AverageCheckInReward = model.TXBMoney(checkIn)
	if result.GroupMessagesTotal, err = integerQuery(ctx, s.db, `SELECT
		(SELECT COUNT(*) FROM activity_group_message_raw_events)+
		(SELECT COALESCE(SUM(group_message_count),0) FROM activity_daily_rollups)`); err != nil {
		return result, err
	}
	if result.PaymentStatuses, err = s.paymentStatusShares(ctx); err != nil {
		return result, err
	}
	if result.ComboShares, result.AverageOptionalSquads, result.SquadByCombo, result.ComboBySquad, err = s.activeCatalogStatistics(ctx, now); err != nil {
		return result, err
	}
	size, err := databaseDiskBytes(ctx, s.db)
	result.DatabaseBytes = strconv.FormatInt(size, 10)
	return result, err
}

func (s *Store) spendStatistics(ctx context.Context) (int64, int64, int64, error) {
	row := s.db.QueryRowContext(ctx, `WITH spend_flows AS (
		SELECT ledger.user_id,-ledger.delta_txb_minor amount FROM ledger_entries ledger
		JOIN users member ON member.id=ledger.user_id WHERE member.role='user'
		AND ledger.delta_txb_minor<0 AND ledger.kind IN ('purchase_debit','automatic_renewal','traffic_reset_debit','emby_setup_debit')
		UNION ALL
		SELECT ledger.user_id,-ledger.delta_txb_minor amount FROM ledger_entries ledger
		JOIN users member ON member.id=ledger.user_id WHERE member.role='user'
		AND ledger.delta_txb_minor>0 AND ledger.kind IN ('purchase_cancellation','admin_entitlement_cancellation',
			'admin_entitlement_refund','member_refund_credit','traffic_reset_compensation','emby_setup_refund')),
	user_spend AS (SELECT user_id,SUM(amount) total FROM spend_flows GROUP BY user_id HAVING SUM(amount)>0)
	SELECT COALESCE(ROUND(AVG(total)),0),COALESCE(MIN(total),0),COALESCE(MAX(total),0) FROM user_spend`)
	var average, minimum, maximum int64
	err := row.Scan(&average, &minimum, &maximum)
	return average, minimum, maximum, err
}

func (s *Store) subscriptionStateShares(ctx context.Context, now time.Time) ([]model.NamedShare, error) {
	rows, err := s.db.QueryContext(ctx, `WITH classified AS (
		SELECT u.id,CASE
		WHEN NOT EXISTS (SELECT 1 FROM purchases p WHERE p.user_id=u.id AND p.status IN ('active','activating') AND p.valid_from<=? AND p.valid_until>?) THEN 'no_active'
		WHEN EXISTS (SELECT 1 FROM purchases p WHERE p.user_id=u.id AND p.status='queued') THEN 'queued'
		WHEN EXISTS (SELECT 1 FROM purchases p WHERE p.user_id=u.id AND p.status IN ('active','activating') AND p.valid_from<=? AND p.valid_until>? AND p.auto_renew_enabled=1) THEN 'auto_renew'
		ELSE 'neither' END state FROM users u WHERE u.role='user')
		SELECT state,COUNT(*) FROM classified GROUP BY state ORDER BY state`, stamp(now), stamp(now), stamp(now), stamp(now))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]model.NamedShare, 0, 4)
	for rows.Next() {
		var id string
		var count float64
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		result = append(result, model.NamedShare{ID: id, Label: id, Value: count})
	}
	return result, rows.Err()
}

func aggregateAverageMinor(ctx context.Context, db *sql.DB, query string) (int64, error) {
	var total, count int64
	if err := db.QueryRowContext(ctx, query).Scan(&total, &count); err != nil || count == 0 {
		return 0, err
	}
	return (total + count/2) / count, nil
}

func integerQuery(ctx context.Context, db *sql.DB, query string, args ...any) (int64, error) {
	var value int64
	err := db.QueryRowContext(ctx, query, args...).Scan(&value)
	return value, err
}

func databaseDiskBytes(ctx context.Context, db *sql.DB) (int64, error) {
	if IsPostgres(db) {
		return integerQuery(ctx, db, `SELECT pg_database_size(current_database())`)
	}
	rows, err := db.QueryContext(ctx, `PRAGMA database_list`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	var path string
	for rows.Next() {
		var sequence int
		var name, candidate string
		if err := rows.Scan(&sequence, &name, &candidate); err != nil {
			return 0, err
		}
		if name == "main" {
			path = candidate
		}
	}
	if path == "" {
		return 0, fmt.Errorf("main database path is unavailable")
	}
	var total int64
	for _, candidate := range []string{path, path + "-wal"} {
		info, statErr := os.Stat(candidate)
		if statErr == nil {
			total += info.Size()
		} else if !os.IsNotExist(statErr) {
			return 0, statErr
		}
	}
	return total, nil
}

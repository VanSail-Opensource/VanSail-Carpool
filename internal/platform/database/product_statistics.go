package database

import (
	"context"
	"database/sql"
	"math/big"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ProductDatabaseStatistics calculates range-free metrics in one read snapshot.
func (s *Store) ProductDatabaseStatistics(ctx context.Context, now time.Time) (model.DatabaseStatistics, error) {
	result := model.DatabaseStatistics{}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	totalUsers, err := integerQuery(ctx, tx, `SELECT COUNT(*) FROM users WHERE role='user'`)
	if err != nil {
		return result, err
	}
	buyers, err := integerQuery(ctx, tx, `SELECT COUNT(*) FROM users member WHERE member.role='user' AND (
		EXISTS (SELECT 1 FROM purchases purchase WHERE purchase.user_id=member.id)
		OR EXISTS (SELECT 1 FROM purchase_history_tombstones purchase WHERE purchase.user_id=member.id))`)
	if err != nil {
		return result, err
	}
	if totalUsers > 0 {
		result.NewUserConversion = float64(buyers) * 100 / float64(totalUsers)
	}
	average, minimum, maximum, err := spendStatisticsTx(ctx, tx)
	if err != nil {
		return result, err
	}
	result.AverageSpend, result.SpendMinimum, result.SpendMaximum = model.TXBMoney(average), model.TXBMoney(minimum), model.TXBMoney(maximum)
	if result.SubscriptionStates, err = subscriptionStateSharesTx(ctx, tx, now); err != nil {
		return result, err
	}
	checkIn, err := aggregateAverageMinor(ctx, tx, `SELECT COALESCE(SUM(total_minor),0),COALESCE(SUM(fact_count),0) FROM (
		SELECT COALESCE(SUM(reward_minor),0) total_minor,COUNT(*) fact_count FROM activity_daily_checkins
		UNION ALL
		SELECT COALESCE(SUM(checkin_reward_txb_minor),0),COALESCE(SUM(checkin_count),0) FROM activity_daily_rollups)`)
	if err != nil {
		return result, err
	}
	result.AverageCheckInReward = model.TXBMoney(checkIn)
	if result.GroupMessagesTotal, err = integerQuery(ctx, tx, `SELECT
		(SELECT COUNT(*) FROM activity_group_message_raw_events)+
		(SELECT COALESCE(SUM(group_message_count),0) FROM activity_daily_rollups)`); err != nil {
		return result, err
	}
	if result.PaymentStatuses, err = paymentStatusSharesTx(ctx, tx); err != nil {
		return result, err
	}
	if result.ComboShares, result.AverageOptionalSquads, result.SquadByCombo, result.ComboBySquad, err = activeCatalogStatisticsTx(ctx, tx, now); err != nil {
		return result, err
	}
	size, err := integerQuery(ctx, tx, `SELECT pg_database_size(current_database())`)
	if err != nil {
		return result, err
	}
	result.DatabaseBytes = strconv.FormatInt(size, 10)
	return result, tx.Commit()
}

func spendStatisticsTx(ctx context.Context, tx *sql.Tx) (int64, int64, int64, error) {
	row := tx.QueryRowContext(ctx, `WITH spend_flows AS (
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

func subscriptionStateSharesTx(ctx context.Context, tx *sql.Tx, now time.Time) ([]model.NamedShare, error) {
	rows, err := tx.QueryContext(ctx, `WITH classified AS (
		SELECT u.id,CASE
		WHEN NOT EXISTS (SELECT 1 FROM purchases p WHERE p.user_id=u.id AND p.status IN ('active','activating') AND `+purchaseTimestampOrderSQL("p.valid_from")+`<=$1 AND `+purchaseTimestampOrderSQL("p.valid_until")+`>$2) THEN 'no_active'
		WHEN EXISTS (SELECT 1 FROM purchases p WHERE p.user_id=u.id AND p.status='queued') THEN 'queued'
		WHEN EXISTS (SELECT 1 FROM purchases p WHERE p.user_id=u.id AND p.status IN ('active','activating') AND `+purchaseTimestampOrderSQL("p.valid_from")+`<=$3 AND `+purchaseTimestampOrderSQL("p.valid_until")+`>$4 AND p.auto_renew_enabled=1) THEN 'auto_renew'
		ELSE 'neither' END state FROM users u WHERE u.role='user')
		SELECT state,COUNT(*) FROM classified GROUP BY state ORDER BY state`, purchaseTimestampOrder(now), purchaseTimestampOrder(now), purchaseTimestampOrder(now), purchaseTimestampOrder(now))
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

type statisticRowReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func aggregateAverageMinor(ctx context.Context, db statisticRowReader, query string) (int64, error) {
	// SUM(bigint) is numeric: its total can exceed int64 while the average fits.
	var rawTotal, rawCount string
	if err := db.QueryRowContext(ctx, query).Scan(&rawTotal, &rawCount); err != nil {
		return 0, err
	}
	total, err := usageUnsigned(rawTotal)
	if err != nil {
		return 0, err
	}
	count, err := usageUnsigned(rawCount)
	if err != nil || count.Sign() == 0 {
		return 0, err
	}
	average, remainder := new(big.Int), new(big.Int)
	average.QuoRem(total, count, remainder)
	if remainder.Lsh(remainder, 1).Cmp(count) >= 0 {
		average.Add(average, big.NewInt(1))
	}
	if !average.IsInt64() {
		return 0, ErrConflict
	}
	return average.Int64(), nil
}

func integerQuery(ctx context.Context, db statisticRowReader, query string, args ...any) (int64, error) {
	var value int64
	err := db.QueryRowContext(ctx, query, args...).Scan(&value)
	return value, err
}

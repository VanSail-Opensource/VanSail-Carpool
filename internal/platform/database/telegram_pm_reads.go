package database

import (
	"context"
	"database/sql"
	"strconv"
	"time"
)

func markPriorPMDeliveriesReadTx(ctx context.Context, tx *sql.Tx, conversationID string, replyToID int64, occurredAt time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT o.id,r.provider_reference FROM provider_operations o
		JOIN provider_operation_items t ON t.operation_id=o.id AND t.item_key='topic' AND t.target_type='pm_conversation'
		JOIN provider_operation_items r ON r.operation_id=o.id AND r.item_key='relay'
		WHERE o.kind='telegram_pm_relay' AND o.status='succeeded' AND t.target_id=$1
		AND ($2 > 0 OR o.created_at<=$3) AND r.status='succeeded' AND r.target_type='pm_outbound_message'
		AND ($4=0 OR r.provider_reference=$5)
		AND NOT EXISTS(SELECT 1 FROM telegram_pm_read_status pm_read WHERE pm_read.operation_id=o.id)
		ORDER BY o.created_at`, conversationID, replyToID, stamp(occurredAt), replyToID, strconv.FormatInt(replyToID, 10))
	if err != nil {
		return err
	}
	type deliveryRef struct{ operationID string }
	deliveries := make([]deliveryRef, 0)
	for rows.Next() {
		var operationID, providerReference string
		if err := rows.Scan(&operationID, &providerReference); err != nil {
			_ = rows.Close()
			return err
		}
		if replyToID > 0 && providerReference != strconv.FormatInt(replyToID, 10) {
			continue
		}
		deliveries = append(deliveries, deliveryRef{operationID: operationID})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, delivery := range deliveries {
		if _, err := tx.ExecContext(ctx, `INSERT   INTO telegram_pm_read_status(operation_id,read_at,source)
			VALUES($1,$2,'reply') ON CONFLICT DO NOTHING `, delivery.operationID, stamp(occurredAt)); err != nil {
			return err
		}
	}
	return nil
}

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func compactPaymentsTx(ctx context.Context, tx *sql.Tx, cutoff, now time.Time, counts map[string]int64) error {
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE maintenance_payment_candidates(id TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create payment cleanup set: %w", err)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO maintenance_payment_candidates(id)
		SELECT payment.id FROM payment_orders payment
		WHERE payment.updated_at<? AND (payment.status IN ('paid','expired','failed','refunded') OR payment.cancelled_at IS NOT NULL)
		AND NOT EXISTS (
			SELECT 1 FROM provider_operation_items item JOIN provider_operations operation ON operation.id=item.operation_id
			WHERE item.target_type='payment' AND item.target_id=payment.id
			AND operation.status IN ('queued','processing','pending_review','partial'))
		AND NOT EXISTS (SELECT 1 FROM affiliate_settlements settlement WHERE settlement.payment_order_id=payment.id)
		AND NOT EXISTS (SELECT 1 FROM courtesy_credits credit WHERE credit.payment_order_id=payment.id)
		AND NOT (payment.provider='builtin_usdt' AND payment.paid_at IS NULL
			AND EXISTS (SELECT 1 FROM usdt_quotes quote WHERE quote.order_id=payment.id))
		AND NOT EXISTS (SELECT 1 FROM usdt_transfers transfer WHERE transfer.order_id=payment.id AND transfer.status='review')`, stamp(cutoff))
	if err != nil {
		return fmt.Errorf("select payment cleanup set: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO payment_status_rollups(local_date,provider,status,order_count,txb_minor,updated_at)
		SELECT substr(payment.updated_at,1,10),payment.provider,
			CASE WHEN payment.status='refunded' THEN 'refunded' WHEN payment.status='paid' THEN 'paid'
				WHEN payment.cancelled_at IS NOT NULL THEN 'cancelled' ELSE payment.status END,
			COUNT(*),COALESCE(SUM(payment.txb_minor),0),?
		FROM payment_orders payment JOIN maintenance_payment_candidates candidate ON candidate.id=payment.id
		GROUP BY substr(payment.updated_at,1,10),payment.provider,
			CASE WHEN payment.status='refunded' THEN 'refunded' WHEN payment.status='paid' THEN 'paid'
				WHEN payment.cancelled_at IS NOT NULL THEN 'cancelled' ELSE payment.status END
		ON CONFLICT(local_date,provider,status) DO UPDATE SET
			order_count=payment_status_rollups.order_count+excluded.order_count,txb_minor=payment_status_rollups.txb_minor+excluded.txb_minor,
			updated_at=excluded.updated_at`, stamp(now))
	if err != nil {
		return fmt.Errorf("roll up payment statuses: %w", err)
	}
	if counts["payment_webhooks"], err = deleteCount(ctx, tx, `DELETE FROM webhook_events
		WHERE order_id IN (SELECT id FROM maintenance_payment_candidates)`); err != nil {
		return fmt.Errorf("prune payment webhooks: %w", err)
	}
	if counts["refunds"], err = deleteCount(ctx, tx, `DELETE FROM refunds
		WHERE payment_order_id IN (SELECT id FROM maintenance_payment_candidates)`); err != nil {
		return fmt.Errorf("prune payment refunds: %w", err)
	}
	if counts["payment_orders"], err = deleteCount(ctx, tx, `DELETE FROM payment_orders
		WHERE id IN (SELECT id FROM maintenance_payment_candidates)`); err != nil {
		return fmt.Errorf("prune payment orders: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE maintenance_payment_candidates`); err != nil {
		return fmt.Errorf("drop payment cleanup set: %w", err)
	}
	return nil
}

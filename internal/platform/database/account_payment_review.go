package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func archivePurgedPaymentTx(ctx context.Context, tx *sql.Tx, order model.PaymentOrder, provider, dedupe, payloadHash, trade, charge, event string, now time.Time) error {
	detail, err := json.Marshal(map[string]any{"reason": "ACCOUNT_PURGED", "orderId": order.ID, "provider": provider, "tradeId": trade, "chargeId": charge, "amountMinor": order.TXBMinor, "requiresReview": true})
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES(?,NULL,'payment.purged_account_review','payment',?,?,?) ON CONFLICT(id) DO NOTHING`, "purged-payment:"+order.ID, order.ID, string(detail), stamp(now)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO payment_callback_tombstones(provider,dedupe_key,order_id,payload_hash,received_at,processed_at) VALUES(?,?,?,?,?,?) ON CONFLICT(provider,dedupe_key) DO NOTHING`, provider, dedupe, order.ID, payloadHash, stamp(now), stamp(now)); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM webhook_events WHERE id=?`, event)
	return err
}

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func compactPurchasesTx(ctx context.Context, tx *sql.Tx, cutoff, now time.Time, counts map[string]int64) error {
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE maintenance_purchase_candidates(id TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create purchase cleanup set: %w", err)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO maintenance_purchase_candidates(id)
		SELECT purchase.id FROM purchases purchase
		WHERE purchase.status IN ('expired','cancelled','failed') AND purchase.updated_at<?
		AND NOT EXISTS (SELECT 1 FROM xnode_periods period WHERE period.purchase_id=purchase.id)
		AND NOT EXISTS (SELECT 1 FROM outbox_jobs job WHERE job.status IN ('pending','processing')
			AND json_extract(job.payload,'$.purchaseId')=purchase.id)
		AND NOT EXISTS (SELECT 1 FROM provider_operation_items item
			JOIN provider_operations operation ON operation.id=item.operation_id
			WHERE item.target_type='purchase' AND item.target_id=purchase.id
			AND operation.status IN ('queued','processing','pending_review','partial'))`, stamp(cutoff))
	if err != nil {
		return fmt.Errorf("select purchase cleanup set: %w", err)
	}
	if err = protectPurchaseLineageTx(ctx, tx); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO purchase_history_tombstones(purchase_id,user_id,combo_id,status,
		charged_txb_minor,gross_txb_minor,core_gross_txb_minor,coupon_discount_txb_minor,addon_count,
		traffic_limit_bytes,reset_strategy,valid_from,valid_until,renewal_batch_id,renewal_index,
		auto_renew_source_purchase_id,created_at,removed_at)
		SELECT purchase.id,purchase.user_id,purchase.combo_id,purchase.status,purchase.charged_txb_minor,
		COALESCE(purchase.gross_price_txb_minor,purchase.charged_txb_minor),purchase.core_gross_txb_minor,
		purchase.coupon_discount_txb_minor,(SELECT COUNT(*) FROM purchase_addons addon WHERE addon.purchase_id=purchase.id),
		COALESCE(purchase.entitlement_traffic_limit_bytes,combo.traffic_limit_bytes),
		COALESCE(purchase.entitlement_reset_strategy,combo.reset_strategy),purchase.valid_from,purchase.valid_until,
		COALESCE(purchase.renewal_batch_id,''),purchase.renewal_index,COALESCE(purchase.auto_renew_source_purchase_id,''),
		purchase.created_at,?
		FROM purchases purchase JOIN combos combo ON combo.id=purchase.combo_id
		JOIN maintenance_purchase_candidates candidate ON candidate.id=purchase.id`, stamp(now))
	if err != nil {
		return fmt.Errorf("preserve purchase tombstones: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_extension_credits SET consumed_by_purchase_id=NULL
		WHERE consumed_by_purchase_id IN (SELECT id FROM maintenance_purchase_candidates)`); err != nil {
		return fmt.Errorf("detach consumed extension lineage: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET auto_renew_source_purchase_id=NULL
		WHERE id IN (SELECT id FROM maintenance_purchase_candidates)
		AND auto_renew_source_purchase_id IN (SELECT id FROM maintenance_purchase_candidates)`); err != nil {
		return fmt.Errorf("detach compacted renewal lineage: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM renewal_batches WHERE source_purchase_id IN
		(SELECT id FROM maintenance_purchase_candidates) AND NOT EXISTS (SELECT 1 FROM purchases term
		WHERE term.renewal_batch_id=renewal_batches.id AND NOT EXISTS
		(SELECT 1 FROM maintenance_purchase_candidates candidate WHERE candidate.id=term.id))`); err != nil {
		return fmt.Errorf("prune renewal batches: %w", err)
	}
	if counts["purchases"], err = deleteCount(ctx, tx,
		`DELETE FROM purchases WHERE id IN (SELECT id FROM maintenance_purchase_candidates)`); err != nil {
		return fmt.Errorf("prune purchase history: %w", err)
	}
	counts["purchase_tombstones"] = counts["purchases"]
	if _, err := tx.ExecContext(ctx, `DROP TABLE maintenance_purchase_candidates`); err != nil {
		return fmt.Errorf("drop purchase cleanup set: %w", err)
	}
	return nil
}

package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func invalidatePricingRenewalBasketTx(ctx context.Context, tx *sql.Tx, purchase string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE pricing_discount_spend SET status='RELEASED' WHERE status='RESERVED' AND reference_id IN (SELECT 'renewal:'||id FROM pricing_renewal_quotes WHERE purchase_id=? AND state<>'SETTLED')`, purchase); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE pricing_renewal_quotes SET state='CANCELLED' WHERE purchase_id=? AND state NOT IN ('SETTLED','EXPIRED','CANCELLED')`, purchase)
	return err
}

// A batch cannot multiply a reserved single-term campaign discount for free.
func reservePricingRenewalBatchTx(ctx context.Context, tx *sql.Tx, user, purchase, batch string, terms int, now time.Time) error {
	if terms <= 1 {
		return nil
	}
	var override string
	var amount int64
	err := tx.QueryRowContext(ctx, `SELECT d.override_id,d.discount_minor FROM pricing_discount_spend d JOIN pricing_renewal_quotes q ON d.reference_id='renewal:'||q.id WHERE q.purchase_id=? AND d.status='RESERVED'`, purchase).Scan(&override, &amount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var raw string
	if err = tx.QueryRowContext(ctx, `SELECT proposed_json FROM pricing_admin_overrides WHERE id=? /*pg:FOR UPDATE*/`, override).Scan(&raw); err != nil {
		return err
	}
	var grant PricingIntervention
	if err = json.Unmarshal([]byte(raw), &grant); err != nil {
		return err
	}
	var spent int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(discount_minor),0) FROM pricing_discount_spend WHERE override_id=? AND status IN ('RESERVED','CONSUMED')`, override).Scan(&spent); err != nil {
		return err
	}
	if amount > (grant.Budget-spent)/int64(terms-1) {
		return pricing.VersionConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_discount_spend(override_id,reference_id,user_id,discount_minor,created_at,status) VALUES(?,?,?,?,?,'CONSUMED')`, override, "renewal-batch:"+batch, user, amount*int64(terms-1), pricingStamp(now))
	return err
}

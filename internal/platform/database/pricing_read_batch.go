package database

import (
	"context"
	"database/sql"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"time"
)

type pricingReadBatchKey struct{}

// Publication and settlement must re-read mutable authorization inputs.
func pricingFreshContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, pricingReadBatchKey{}, nil)
}

type pricingReadBatch struct {
	Policies        map[string]pricing.Policy
	Grants          []coupons.Grant
	User            string
	Observation     pricing.ResponseObservation
	FeatureComplete int
	Economics       map[string]string
	Watermark       string
}

// This request-local projection is used only during analytical preparation.
// Commerce transactions never inherit it and re-read every financial precondition.
func (s *Store) retryAttempt_pricingReadContext(ctx context.Context, user string, now time.Time) (context.Context, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ctx, err
	}
	defer tx.Rollback()
	batch := &pricingReadBatch{Policies: map[string]pricing.Policy{}, User: user, Grants: []coupons.Grant{}, Economics: map[string]string{}}
	rows, err := tx.QueryContext(ctx, pricingPolicySelect+" ORDER BY product_type,product_id LIMIT 4097")
	if err != nil {
		return ctx, err
	}
	for rows.Next() {
		p, e := scanPricingPolicy(rows)
		if e != nil {
			rows.Close()
			return ctx, e
		}
		if len(batch.Policies) >= 4096 {
			rows.Close()
			return ctx, pricing.WorkerResourceLimit
		}
		batch.Policies[p.ProductType+":"+p.ProductID] = p
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ctx, err
	}
	if user != "" {
		batch.Observation, batch.FeatureComplete, batch.Watermark, err = pricingExposureInputs(ctx, tx, user, now)
		if err != nil {
			return ctx, err
		}
		rows, err = tx.QueryContext(ctx, grantSelect+` WHERE coupon_grants.user_id=? AND coupon_grants.status='active' AND coupon_definitions.active=1 AND (coupon_definitions.expires_at IS NULL OR coupon_definitions.expires_at>?) AND NOT EXISTS(SELECT 1 FROM coupon_grant_discards d WHERE d.grant_id=coupon_grants.id) AND `+authorizedCouponGrant+` AND coupon_definitions.kind IN ('purchase_once','purchase_recurring') AND (coupon_definitions.global_use_limit IS NULL OR (SELECT COUNT(*) FROM coupon_uses u WHERE u.coupon_id=coupon_definitions.id)<coupon_definitions.global_use_limit) AND (coupon_definitions.per_user_use_limit IS NULL OR (SELECT COUNT(*) FROM coupon_uses u WHERE u.coupon_id=coupon_definitions.id AND u.user_id=coupon_grants.user_id)<coupon_definitions.per_user_use_limit) ORDER BY coupon_grants.id LIMIT 4097`, user, pricingStamp(now))
		if err != nil {
			return ctx, err
		}
		count := 0
		for rows.Next() {
			g, e := scanGrant(rows)
			if e != nil {
				rows.Close()
				return ctx, e
			}
			count++
			if count > 4096 {
				rows.Close()
				return ctx, pricing.WorkerResourceLimit
			}
			if g.Coupon.Kind == coupons.KindPurchaseOnce || g.Coupon.Kind == coupons.KindPurchaseRecurring {
				batch.Grants = append(batch.Grants, g)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return ctx, err
		}

	}
	enabled := false
	for _, p := range batch.Policies {
		enabled = enabled || p.DynamicEnabled && (p.PurchaseMode == "DYNAMIC" || p.PersonalDiscounts)
	}
	if enabled {
		rows, err = tx.QueryContext(ctx, `SELECT product_type,product_id,trace_json FROM (SELECT product_type,product_id,trace_json,ROW_NUMBER() OVER(PARTITION BY product_type,product_id ORDER BY generated_at DESC,id DESC) AS ordinal FROM pricing_decisions WHERE state='PUBLISHED' AND generated_at>=?) latest WHERE ordinal=1 LIMIT 4097`, pricingStamp(now.Add(-24*time.Hour)))
		if err != nil {
			return ctx, err
		}
		for rows.Next() {
			var kind, id, raw string
			if err = rows.Scan(&kind, &id, &raw); err != nil {
				rows.Close()
				return ctx, err
			}
			if len(batch.Economics) >= 4096 {
				rows.Close()
				return ctx, pricing.WorkerResourceLimit
			}
			batch.Economics[kind+":"+id] = raw
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return ctx, err
		}
	}
	return context.WithValue(ctx, pricingReadBatchKey{}, batch), nil
}
func scanPricingPolicy(row rowScanner) (pricing.Policy, error) {
	var p pricing.Policy
	var enabled, personal int
	var updated string
	err := row.Scan(&p.ProductType, &p.ProductID, &enabled, &p.PurchaseMode, &p.RenewalMode, &personal, &p.FixedPrice, &p.MaxIncreaseBPS, &p.MaxDiscountBPS, &p.OverrideStatus, &p.LastApprovedVersion, &p.Version, &p.UpdatedBy, &updated)
	if err != nil {
		return p, err
	}
	p.DynamicEnabled, p.PersonalDiscounts = enabled == 1, personal == 1
	p.UpdatedAt, err = parseStamp(updated)
	return p, err
}

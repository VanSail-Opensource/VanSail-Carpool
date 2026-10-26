package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// resolvePricingTx reads optional pricing within the authoritative transaction.
// It never rewrites a purchased allocation, historical charge or renewal contract.
func resolvePricingTx(ctx context.Context, tx *sql.Tx, kind, id string, fixed int64, now time.Time) (int64, error) {
	p, err := loadPricingPolicy(ctx, tx, kind, id, fixed)
	if err != nil {
		return 0, err
	}
	if !p.DynamicEnabled || p.PurchaseMode != "DYNAMIC" {
		return fixed, pricingOperatorAdmissionTx(ctx, tx, kind, id, now)
	}
	if err = pricingAdmissionTx(ctx, tx, kind, id, now); err != nil {
		return 0, err
	}
	return approvedPricingTx(ctx, tx, p, fixed, now)
}

func approvedPricingTx(ctx context.Context, tx *sql.Tx, p pricing.Policy, fixed int64, now time.Time) (int64, error) {
	var price int64
	// Active manual prices expire naturally. The safe publication pointer remains
	// unchanged, so expiration cannot overwrite a newer authorized policy.
	// Published manual rows are selected only while their approved intervention lives.
	err := tx.QueryRowContext(ctx, `SELECT price_minor FROM pricing_published_prices WHERE product_type=? AND product_id=? AND source='MANUAL' AND policy_version=? AND valid_from<=? AND valid_until>? AND EXISTS(SELECT 1 FROM pricing_admin_overrides o WHERE o.id=pricing_published_prices.id AND o.status='ACTIVE') ORDER BY created_at DESC LIMIT 1`, p.ProductType, p.ProductID, p.Version, pricingStamp(now), pricingStamp(now)).Scan(&price)
	if err == nil {
		return price, nil
	} // Explicit administrator rate takes precedence over inferred bounds.
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `SELECT price_minor FROM pricing_published_prices WHERE id=? AND valid_from<=? AND valid_until>?`, p.LastApprovedVersion, pricingStamp(now), pricingStamp(now)).Scan(&price)
	}
	if errors.Is(err, sql.ErrNoRows) {
		if p.LastApprovedVersion == "" {
			return fixed, nil
		}
		return 0, pricing.ForecastStale
	}
	if err != nil {
		return 0, err
	}
	bounded, err := pricing.BoundPrice(p.FixedPrice, price, p.MaxIncreaseBPS, p.MaxDiscountBPS)
	if err != nil {
		return 0, err
	}
	if bounded != price {
		return 0, pricing.ModelUnvalidated
	}
	return price, nil
}

func pricingAdmissionTx(ctx context.Context, tx *sql.Tx, kind, id string, now time.Time) error {
	if err := pricingOperatorAdmissionTx(ctx, tx, kind, id, now); err != nil {
		return err
	}
	return pricingHealthAdmissionTx(ctx, tx, kind, id, now)
}

func pricingOperatorAdmissionTx(ctx context.Context, tx *sql.Tx, kind, id string, now time.Time) error {
	var blocked int
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_admin_overrides WHERE status='ACTIVE' AND starts_at<=? AND expires_at>? AND command='PAUSE_ADMISSIONS' AND (scope_type='global' OR (scope_type='product' AND scope_id=?)))`, pricingStamp(now), pricingStamp(now), kind+":"+id).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked != 0 {
		return pricing.CapacityUnsafe
	}
	var nodeRules int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_admin_overrides WHERE command='PAUSE_ADMISSIONS' AND scope_type IN ('node','machine') AND status='ACTIVE' AND starts_at<=? AND expires_at>?)`, pricingStamp(now), pricingStamp(now)).Scan(&nodeRules); err != nil {
		return err
	}
	if nodeRules != 0 {
		nodes, err := pricingNodesTx(ctx, tx, kind, id)
		if err != nil {
			return err
		}
		for _, node := range nodes {
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_admin_overrides WHERE command='PAUSE_ADMISSIONS' AND scope_type IN ('node','machine') AND scope_id=? AND status='ACTIVE' AND starts_at<=? AND expires_at>?)`, node, pricingStamp(now), pricingStamp(now)).Scan(&blocked); err != nil {
				return err
			}
			if blocked != 0 {
				return pricing.CapacityUnsafe
			}
		}
	}
	return nil
}

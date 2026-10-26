package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// personalPricingTx treats eligibility and campaign budget as separate gates.
// Propensity alone never grants a discount. Coupon-bearing quotes bypass this.
func personalPricingTx(ctx context.Context, tx *sql.Tx, user, kind, product string, public int64, blocks int64, reference string, reserve bool, now time.Time) (int64, error) {
	p, err := loadPricingPolicy(ctx, tx, kind, product, public)
	if err != nil {
		return 0, err
	}
	if !p.DynamicEnabled || !p.PersonalDiscounts || blocks < 1 {
		return public, nil
	}
	var id, raw string
	err = tx.QueryRowContext(ctx, `SELECT id,proposed_json FROM pricing_admin_overrides WHERE scope_type='user_product' AND scope_id=? AND command='PERSONAL_DISCOUNT' AND status='ACTIVE' AND starts_at<=? AND expires_at>? ORDER BY created_at DESC LIMIT 1`, user+":"+kind+":"+product, pricingStamp(now), pricingStamp(now)).Scan(&id, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return public, nil
	}
	if err != nil {
		return 0, err
	}
	var in PricingIntervention
	if err = json.Unmarshal([]byte(raw), &in); err != nil {
		return 0, err
	}
	if in.DiscountBPS <= 0 || in.DiscountBPS > 10000 {
		return public, nil
	}
	price := public * (10000 - in.DiscountBPS) / 10000
	if kind == "payg" {
		price = max(1, price)
	}
	discount := public - price
	if discount == 0 || blocks > in.Budget/discount {
		return public, nil
	}
	if reserve {
		var locked string
		if err = tx.QueryRowContext(ctx, `SELECT id FROM pricing_admin_overrides WHERE id=? /*pg:FOR UPDATE*/`, id).Scan(&locked); err != nil {
			return 0, err
		}
	}
	var spent int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(discount_minor),0) FROM pricing_discount_spend WHERE override_id=? AND status IN ('RESERVED','CONSUMED')`, id).Scan(&spent); err != nil {
		return 0, err
	}
	amount := discount * blocks
	if spent > in.Budget-amount {
		return public, nil
	}
	if reserve {
		status := "CONSUMED"
		if strings.HasPrefix(reference, "renewal:") {
			status = "RESERVED"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO pricing_discount_spend(override_id,reference_id,user_id,discount_minor,created_at,status) VALUES(?,?,?,?,?,?)`, id, reference, user, amount, pricingStamp(now), status)
		if err != nil {
			return 0, err
		}
	}
	return price, nil
}

func reservePurchaseDiscountTx(ctx context.Context, tx *sql.Tx, user, kind, product, purchase string, selected, public int64, now time.Time) error {
	p, err := loadPricingPolicy(ctx, tx, kind, product, selected)
	if err != nil {
		return err
	}
	if !p.DynamicEnabled || !p.PersonalDiscounts {
		return nil
	}
	price, err := personalPricingTx(ctx, tx, user, kind, product, public, 1, purchase+":"+kind+":"+product, true, now)
	if err != nil {
		return err
	}
	if price != selected {
		return pricing.VersionConflict
	}
	return nil
}

func paygPricingTx(ctx context.Context, tx *sql.Tx, user, group string, public int64, blocks, principal, first int64, now time.Time) (int64, int64, error) {
	p, err := loadPricingPolicy(ctx, tx, "payg", group, public)
	if err != nil {
		return 0, 0, err
	}
	if !p.DynamicEnabled || !p.PersonalDiscounts {
		return public, public, nil
	}
	price, err := personalPricingTx(ctx, tx, user, "payg", group, public, blocks, fmt.Sprintf("payg:%d:%d", principal, first), true, now)
	return public, price, err
}

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

// AutoRenewalPlan returns current local pricing and availability for an owned term.
func (s *Store) AutoRenewalPlan(ctx context.Context, userID, purchaseID string, now time.Time) (AutoRenewalPlan, error) {
	return s.autoRenewalPlan(ctx, userID, purchaseID, nil, now)
}

// AutoRenewalPlanExcludingAddons replans a renewal after live add-on validation.
func (s *Store) AutoRenewalPlanExcludingAddons(ctx context.Context, userID, purchaseID string, excludedAddonIDs []string, now time.Time) (AutoRenewalPlan, error) {
	return s.autoRenewalPlan(ctx, userID, purchaseID, excludedAddonIDs, now)
}

func (s *Store) retryAttempt_autoRenewalPlan(ctx context.Context, userID, purchaseID string, excludedAddonIDs []string, now time.Time) (AutoRenewalPlan, error) {
	ctx = pricing.WithHealth(ctx, s.pricingHealth)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return AutoRenewalPlan{}, fmt.Errorf("begin automatic renewal quote: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	return automaticRenewalPlanTx(ctx, tx, userID, purchaseID, excludedAddonIDs, now.UTC())
}

func automaticRenewalPlanTx(ctx context.Context, tx *sql.Tx, userID, purchaseID string, excludedAddonIDs []string, now time.Time) (AutoRenewalPlan, error) {
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=$1 AND purchases.user_id=$2`, purchaseID, userID))
	if err != nil {
		return AutoRenewalPlan{}, err
	}
	plan := AutoRenewalPlan{Purchase: purchase, GrossMinor: purchase.GrossPriceTXBMinor,
		DiscountMinor: purchase.CouponDiscountTXBMinor, NetMinor: purchase.PriceTXBMinor, ScheduledAt: purchase.ValidUntil}
	if duration := purchase.ValidUntil.Sub(purchase.ValidFrom); duration > 0 {
		plan.NextCycleEndsAt = plan.ScheduledAt.Add(duration)
	} else {
		plan.NextCycleEndsAt = plan.ScheduledAt
	}
	var trafficLimit, renewalPrice, renewalTraffic sql.NullInt64
	var resetStrategy, squadUUIDs sql.NullString
	var trafficRenewal int
	if err = tx.QueryRowContext(ctx, `SELECT entitlement_traffic_limit_bytes,entitlement_reset_strategy,entitlement_squad_uuids,
		reward_renewal_price_minor,reward_traffic_renewal,
		reward_renewal_traffic_limit_bytes FROM purchases WHERE id=$1`, purchaseID).Scan(
		&trafficLimit, &resetStrategy, &squadUUIDs, &renewalPrice, &trafficRenewal, &renewalTraffic); err != nil {
		return AutoRenewalPlan{}, fmt.Errorf("load renewal entitlement overrides: %w", err)
	}
	if trafficRenewal == 0 {
		trafficLimit = renewalTraffic
	}
	plan.rewardTrafficRenewal = true
	if trafficLimit.Valid {
		value := trafficLimit.Int64
		plan.trafficLimitOverride = &value
	}
	if resetStrategy.Valid {
		value := resetStrategy.String
		plan.resetStrategyOverride = &value
	}
	if squadUUIDs.Valid {
		value := squadUUIDs.String
		plan.squadUUIDsOverride = &value
	}
	if renewalPrice.Valid {
		value := renewalPrice.Int64
		plan.rewardRenewalPrice = &value
	}
	if purchase.Status != "active" && purchase.Status != "activating" {
		if purchase.Status != "expired" {
			plan.IneligibleReason = AutoRenewalReasonPurchaseUnavailable
			return plan, nil
		}
	}
	queued, err := otherRenewalPurchaseEndsAfter(ctx, tx, userID, purchaseID, now)
	if err != nil {
		return AutoRenewalPlan{}, fmt.Errorf("check queued purchase: %w", err)
	}
	if queued {
		plan.IneligibleReason = AutoRenewalReasonQueuedPurchase
	}
	// Owned renewal: a hidden or inactive combo stays renewable (the same
	// resolution the manual quote uses); a missing row fails closed. The
	// purchase's full override or recorded base snapshot decides its squads.
	combo, err := ownedRenewalComboTx(ctx, tx, purchase.ID)
	if errors.Is(err, ErrNotFound) {
		plan.IneligibleReason = AutoRenewalReasonComboUnavailable
		return plan, nil
	}
	if err != nil {
		return AutoRenewalPlan{}, err
	}
	plan.Combo = combo
	if plan.squadUUIDsOverride != nil {
		value := squadJSON(plan.Combo.IncludedSquads)
		plan.squadUUIDsOverride = &value
	}
	// Awarded hours extend only the current term; renewals use the selected core combo's cadence.
	plan.NextCycleEndsAt = plan.ScheduledAt.AddDate(0, 0, combo.ValidityDays)
	plan.GrossMinor, plan.DiscountMinor, plan.NetMinor = combo.PriceTXBMinor, 0, combo.PriceTXBMinor
	if plan.rewardRenewalPrice != nil {
		plan.GrossMinor, plan.NetMinor = *plan.rewardRenewalPrice, *plan.rewardRenewalPrice
	}
	addons, err := renewalAddonsTx(ctx, tx, purchase.ID, excludedAddonIDs)
	if err != nil {
		return AutoRenewalPlan{}, err
	}
	if plan.rewardRenewalPrice != nil {
		addons = nil
	}
	plan.Addons = addons
	if empty, e := emptyRenewalTx(ctx, tx, purchase.ID); e != nil {
		return AutoRenewalPlan{}, e
	} else if empty {
		plan.IneligibleReason = AutoRenewalReasonNoAccessibleNodes
		return plan, nil
	}
	addonIDs := make([]string, 0, len(addons))
	for _, addon := range addons {
		plan.GrossMinor, err = checkedPriceSum(plan.GrossMinor, addon.PriceTXBMinor)
		if err != nil {
			return AutoRenewalPlan{}, err
		}
		addonIDs = append(addonIDs, addon.GroupID)
	}
	if plan.rewardRenewalPrice == nil {
		plan.GrossMinor, err = pricingRenewalGrossTx(ctx, tx, purchaseID, plan.GrossMinor, addons, now)
		if err != nil {
			return AutoRenewalPlan{}, err
		}
		plan.Combo.PriceTXBMinor = plan.GrossMinor
		for _, addon := range addons {
			if addon.PriceTXBMinor < 0 || plan.Combo.PriceTXBMinor < addon.PriceTXBMinor {
				return AutoRenewalPlan{}, ErrConflict
			}
			plan.Combo.PriceTXBMinor -= addon.PriceTXBMinor
		}
		if plan.Combo.PriceTXBMinor < 0 {
			return AutoRenewalPlan{}, ErrConflict
		}
	}
	if plan.GrossMinor < 0 {
		return AutoRenewalPlan{}, ErrConflict
	}
	plan.NetMinor = plan.GrossMinor
	discount := coupons.Discount{GrossMinor: plan.GrossMinor, NetMinor: plan.GrossMinor}
	if purchase.RecurringDiscountAttached {
		if purchase.CouponGrantID == nil {
			plan.IneligibleReason = AutoRenewalReasonDiscountUnavailable
			return plan, nil
		}
		discount, err = quoteAttachedRecurringDiscountTx(ctx, tx, userID, *purchase.CouponGrantID, plan.GrossMinor)
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) || errors.Is(err, coupons.ErrInvalidInput) {
			plan.IneligibleReason = AutoRenewalReasonDiscountUnavailable
			return plan, nil
		}
		if err != nil {
			return AutoRenewalPlan{}, err
		}
	}
	plan.DiscountMinor, plan.NetMinor = discount.DiscountMinor, discount.NetMinor
	if err := checkSquadStockTx(ctx, tx, addonIDs, userID); errors.Is(err, ErrStockUnavailable) {
		plan.IneligibleReason = AutoRenewalReasonPaidAddonUnavailable
	} else if err != nil {
		return AutoRenewalPlan{}, err
	}
	return plan, nil
}

package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
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
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=? AND purchases.user_id=?`, purchaseID, userID))
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
		reward_renewal_traffic_limit_bytes FROM purchases WHERE id=?`, purchaseID).Scan(
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
	var queued int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=?
		AND status IN ('active','activating','queued') AND id<>? AND valid_until>?)`, userID, purchaseID, stamp(now)).Scan(&queued); err != nil {
		return AutoRenewalPlan{}, fmt.Errorf("check queued purchase: %w", err)
	}
	if queued == 1 {
		plan.IneligibleReason = AutoRenewalReasonQueuedPurchase
	}
	combo, err := comboByIDTx(ctx, tx, purchase.ComboID, true)
	if errors.Is(err, ErrNotFound) {
		plan.IneligibleReason = AutoRenewalReasonComboUnavailable
		return plan, nil
	}
	if err != nil {
		return AutoRenewalPlan{}, err
	}
	plan.Combo = combo
	// Full entitlement overrides replace the base combo's squads at renewal too.
	if squadUUIDs.Valid {
		var ids []string
		if err := json.Unmarshal([]byte(squadUUIDs.String), &ids); err != nil {
			return AutoRenewalPlan{}, fmt.Errorf("decode renewal squad overrides: %w", err)
		}
		plan.Combo.IncludedSquads = make([]model.SquadProduct, 0, len(ids))
		for _, id := range uniqueSorted(ids) {
			plan.Combo.IncludedSquads = append(plan.Combo.IncludedSquads, model.SquadProduct{ID: id, GroupID: id})
		}
	}
	plan.Combo, err = renewalComboTx(ctx, tx, purchase.ID, plan.Combo)
	if err != nil {
		return AutoRenewalPlan{}, err
	}
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
		plan.GrossMinor += addon.PriceTXBMinor
		addonIDs = append(addonIDs, addon.GroupID)
	}
	if plan.rewardRenewalPrice == nil {
		plan.GrossMinor, err = pricingRenewalGrossTx(ctx, tx, purchaseID, plan.GrossMinor, addons, now)
		if err != nil {
			return AutoRenewalPlan{}, err
		}
		plan.Combo.PriceTXBMinor = plan.GrossMinor
		for _, addon := range addons {
			plan.Combo.PriceTXBMinor -= addon.PriceTXBMinor
		}
		if plan.Combo.PriceTXBMinor < 0 {
			return AutoRenewalPlan{}, ErrConflict
		}
	}
	plan.NetMinor = plan.GrossMinor
	discount := coupons.Discount{GrossMinor: plan.GrossMinor, NetMinor: plan.GrossMinor}
	if purchase.RecurringDiscountAttached {
		if purchase.CouponGrantID == nil {
			plan.IneligibleReason = AutoRenewalReasonDiscountUnavailable
			return plan, nil
		}
		discount, err = quoteAttachedRecurringDiscountTx(ctx, tx, userID, *purchase.CouponGrantID, plan.GrossMinor)
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) {
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

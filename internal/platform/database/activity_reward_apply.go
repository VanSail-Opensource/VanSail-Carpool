package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func resolveDrawReward(spec activity.Reward, rng activity.RandomSource) (activity.Reward, error) {
	// Recheck stored specifications too: older definitions may predate validation.
	if err := spec.Validate(); err != nil {
		return activity.Reward{}, err
	}
	if spec.Range == nil {
		return spec, nil
	}
	var value int64
	var err error
	if spec.Kind == activity.RewardBalanceMultiplier {
		value, err = spec.Range.RollAround(rng, 10000)
	} else {
		value, err = spec.Range.Roll(rng)
	}
	if err != nil {
		return activity.Reward{}, err
	}
	spec.ResolvedValue = &value
	return spec, nil
}

func activeRewardPurchase(ctx context.Context, tx *sql.Tx, userID string, now time.Time) (string, int64, error) {
	var id string
	var traffic int64
	err := tx.QueryRowContext(ctx, `SELECT purchases.id,COALESCE(purchases.entitlement_traffic_limit_bytes,combos.traffic_limit_bytes)
  FROM purchases JOIN combos ON combos.id=purchases.combo_id WHERE purchases.user_id=?
  AND purchases.status IN ('active','activating') AND purchases.valid_from<=? AND purchases.valid_until>?
  ORDER BY purchases.valid_from DESC LIMIT 1`, userID, stamp(now), stamp(now)).Scan(&id, &traffic)
	if err == sql.ErrNoRows {
		return "", 0, ErrConflict
	}
	return id, traffic, err
}

func applyDrawRewardTx(ctx context.Context, tx *sql.Tx, userID, resultID, prizeName string, reward activity.Reward, balance int64, now time.Time) (int64, error) {
	value := int64(0)
	if reward.ResolvedValue != nil {
		value = *reward.ResolvedValue
	}
	switch reward.Kind {
	case activity.RewardNone:
		return balance, nil
	case activity.RewardTXBDelta:
		if reward.Range == nil {
			value = reward.TXBDeltaMinor
		}
		next, err := changeBalanceTx(ctx, tx, userID, value, now)
		if err != nil {
			return 0, err
		}
		if _, err = insertLedgerTx(ctx, tx, userID, value, next, "activity_draw_reward", resultID, prizeName, now); err != nil {
			return 0, err
		}
		return next, nil
	case activity.RewardBalanceMultiplier:
		next, err := fixedMultiplyFloor(balance, value, 10000)
		if err != nil {
			return 0, err
		}
		delta := next - balance
		if delta == 0 {
			return balance, nil
		}
		next, err = changeBalanceTx(ctx, tx, userID, delta, now)
		if err != nil {
			return 0, err
		}
		if _, err = insertLedgerTx(ctx, tx, userID, delta, next, "activity_draw_reward", resultID, prizeName, now); err != nil {
			return 0, err
		}
		return next, nil
	case activity.RewardCouponRecurring, activity.RewardCouponOnce:
		return balance, grantDrawCouponTx(ctx, tx, userID, resultID, prizeName, reward, value, now)
	case activity.RewardSubscriptionExtension:
		if reward.Range == nil {
			value = int64(reward.ExtensionDays) * 24
		}
		return balance, applySubscriptionExtensionHoursTx(ctx, tx, userID, int(value), "activity_draw", resultID, now)
	case activity.RewardEntitlementGrant, activity.RewardSquadAccess, activity.RewardCoreComboSwitch,
		activity.RewardTrafficGrant, activity.RewardTrafficReset:
		return balance, applyDrawEntitlementTx(ctx, tx, userID, resultID, reward, value, now)
	case activity.RewardCouponGrant: // immutable historical results remain readable
		coupon, err := couponByID(ctx, tx, reward.CouponID)
		if err != nil {
			return 0, err
		}
		_, err = grantCouponTx(ctx, tx, userID, coupon, "activity_draw", resultID, now)
		return balance, err
	default:
		return 0, activity.ErrInvalidInput
	}
}

func applyDrawEntitlementTx(ctx context.Context, tx *sql.Tx, userID, resultID string, reward activity.Reward, value int64, now time.Time) error {
	purchaseID, traffic, err := activeRewardPurchase(ctx, tx, userID, now)
	if err != nil {
		return err
	}
	switch reward.Kind {
	case activity.RewardEntitlementGrant:
		squads, marshalErr := json.Marshal(reward.SquadUUIDs)
		if marshalErr != nil {
			return marshalErr
		}
		combo, comboErr := comboByIDTx(ctx, tx, reward.ComboID, true)
		if comboErr != nil {
			return comboErr
		}
		_, err = tx.ExecContext(ctx, `UPDATE purchases SET combo_id=?,entitlement_traffic_limit_bytes=?,entitlement_squad_uuids=?,
   entitlement_addon_squad_uuids='[]',reward_renewal_price_minor=?,
   reward_traffic_renewal=1,reward_renewal_traffic_limit_bytes=?,entitlement_reset_strategy=?,updated_at=? WHERE id=?`, reward.ComboID, reward.TrafficLimitBytes, string(squads),
			reward.RenewalPriceMinor, reward.TrafficLimitBytes, combo.ResetStrategy, stamp(now), purchaseID)
		if err == nil {
			err = nil
		}
	case activity.RewardSquadAccess:
		for _, squad := range reward.SquadUUIDs {
			if squad == "" {
				return activity.ErrInvalidInput
			}
			_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO activity_reward_squad_access(purchase_id,squad_uuid,source_result_id) VALUES(?,?,?)`, purchaseID, squad, resultID)
			if err != nil {
				return err
			}
		}
	case activity.RewardCoreComboSwitch:
		err = applyDrawCoreComboTx(ctx, tx, purchaseID, reward.ComboID, now)
	case activity.RewardTrafficGrant:
		err = applyDrawTrafficTx(ctx, tx, purchaseID, traffic, value, reward.IncludeInRenewal, now)
	case activity.RewardTrafficReset:
		_, err = tx.ExecContext(ctx, `UPDATE purchases SET status='activating',traffic_reset_phase='pending',updated_at=? WHERE id=?`, stamp(now), purchaseID)
		if err == nil {
			return insertOutboxTx(ctx, tx, "xnode_apply_entitlement", `{"purchaseId":"`+purchaseID+`"}`, now, now)
		}
	}
	if err != nil {
		return fmt.Errorf("apply draw entitlement: %w", err)
	}
	return insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+userID+`"}`, now, now)
}

func grantDrawCouponTx(ctx context.Context, tx *sql.Tx, userID, resultID, name string, reward activity.Reward, value int64, now time.Time) error {
	couponID, err := ids.New()
	if err != nil {
		return err
	}
	kind := "purchase_once"
	if reward.Kind == activity.RewardCouponRecurring {
		kind = "purchase_recurring"
	}
	code := "DRAW_" + strings.ToUpper(couponID)
	_, err = tx.ExecContext(ctx, `INSERT INTO coupon_definitions(id,code,name,kind,discount_mode,value_minor_or_bps,
  eligible_combo_ids,eligible_squad_ids,active,created_at,updated_at,admin_visible)
  VALUES(?,?,?,?,?,?,'[]','[]',1,?,?,0)`, couponID, code, name, kind, reward.DiscountMode, value, stamp(now), stamp(now))
	if err != nil {
		return err
	}
	coupon, err := couponByID(ctx, tx, couponID)
	if err != nil {
		return err
	}
	_, err = grantCouponTx(ctx, tx, userID, coupon, "activity_draw", resultID, now)
	return err
}

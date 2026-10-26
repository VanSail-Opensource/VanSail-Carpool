package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func (s *Store) commitAutoRenewal(ctx context.Context, purchaseID string, excludedAddonIDs []string, now time.Time) (model.Purchase, error) {
	ctx = pricing.WithHealth(ctx, s.pricingHealth)
	var replay int
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE auto_renew_source_purchase_id=?)`, purchaseID).Scan(&replay); err != nil {
		return model.Purchase{}, err
	}
	if replay == 0 {
		if err := s.prepareOwnedPricingRenewal(ctx, "", purchaseID, now); err != nil {
			return model.Purchase{}, err
		}
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Purchase{}, fmt.Errorf("begin automatic renewal: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if successorID, found, err := automaticRenewalSuccessorIDTx(ctx, tx, purchaseID); err != nil {
		return model.Purchase{}, err
	} else if found {
		if err := tx.Rollback(); err != nil {
			return model.Purchase{}, err
		}
		return s.PurchaseByID(ctx, successorID)
	}
	var userID string
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM purchases WHERE id=?`, purchaseID).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Purchase{}, ErrNotFound
		}
		return model.Purchase{}, fmt.Errorf("load automatic renewal owner: %w", err)
	}
	if err := lockMemberPurchaseTx(ctx, tx, userID); err != nil {
		return model.Purchase{}, err
	}
	plan, err := automaticRenewalPlanTx(ctx, tx, userID, purchaseID, excludedAddonIDs, now)
	if err != nil {
		return model.Purchase{}, err
	}
	if !plan.Purchase.AutoRenewEnabled || plan.ScheduledAt.After(now) || plan.IneligibleReason != "" {
		return model.Purchase{}, ErrConflict
	}
	if err := validatePricingRenewalSettlementTx(ctx, tx, purchaseID, plan.Addons, true, now); err != nil {
		return model.Purchase{}, err
	}
	balance, err := balanceTx(ctx, tx, userID)
	if err != nil {
		return model.Purchase{}, err
	}
	if balance < plan.NetMinor {
		if _, err = s.failAutoRenewalTx(ctx, tx, purchaseID, userID, AutoRenewalReasonInsufficientBalance, now); err != nil {
			return model.Purchase{}, err
		}
		if err = tx.Commit(); err != nil {
			return model.Purchase{}, err
		}
		return model.Purchase{}, ErrInsufficientBalance
	}
	newBalance, err := debitBalanceTx(ctx, tx, userID, plan.NetMinor, now)
	if err != nil {
		return model.Purchase{}, err
	}
	successorID, err := ids.New()
	if err != nil {
		return model.Purchase{}, err
	}
	if err := insertAutomaticRenewalSuccessorTx(ctx, tx, successorID, purchaseID, userID, plan, now); err != nil {
		return model.Purchase{}, err
	}
	if err := inheritPricingContractTx(ctx, tx, purchaseID, successorID, plan.GrossMinor, now); err != nil {
		return model.Purchase{}, err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='expired',auto_renew_enabled=0,updated_at=? WHERE id=? AND status IN ('active','activating','expired')`, stamp(now), purchaseID); err != nil {
		return model.Purchase{}, err
	}
	if err := enqueuePurchaseTransitionTx(ctx, tx, successorID, "activating", plan.ScheduledAt, now); err != nil {
		return model.Purchase{}, err
	}
	if _, err := insertLedgerTx(ctx, tx, userID, -plan.NetMinor, newBalance, "automatic_renewal", successorID, plan.Combo.Name, now); err != nil {
		return model.Purchase{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, fmt.Errorf("commit automatic renewal: %w", err)
	}
	return s.PurchaseByID(ctx, successorID)
}

func insertAutomaticRenewalSuccessorTx(ctx context.Context, tx *sql.Tx, successorID, purchaseID, userID string, plan AutoRenewalPlan, now time.Time) error {
	var couponGrantID any
	coreGross := plan.Combo.PriceTXBMinor
	if plan.rewardRenewalPrice != nil {
		coreGross = *plan.rewardRenewalPrice
	}
	if plan.Purchase.RecurringDiscountAttached && plan.Purchase.CouponGrantID != nil {
		couponGrantID = *plan.Purchase.CouponGrantID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,valid_from,valid_until,status,coupon_grant_id,
		gross_price_txb_minor,core_gross_txb_minor,coupon_discount_txb_minor,auto_renew_enabled,recurring_discount_attached,auto_renew_source_purchase_id,request_fingerprint,
		entitlement_traffic_limit_bytes,entitlement_reset_strategy,entitlement_squad_uuids,reward_renewal_price_minor,
		reward_traffic_renewal,reward_renewal_traffic_limit_bytes,created_at,updated_at)
		VALUES(?,?,?,?,?,?,'activating',?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, successorID, userID, plan.Combo.ID, plan.NetMinor,
		stamp(plan.ScheduledAt), stamp(plan.NextCycleEndsAt), couponGrantID, plan.GrossMinor, coreGross, plan.DiscountMinor, 1,
		boolInt(plan.Purchase.RecurringDiscountAttached), purchaseID, "automatic-renewal:"+purchaseID, plan.trafficLimitOverride, plan.resetStrategyOverride,
		plan.squadUUIDsOverride, plan.rewardRenewalPrice, boolInt(plan.rewardTrafficRenewal),
		plan.trafficLimitOverride, stamp(now), stamp(now))
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrConflict
		}
		return fmt.Errorf("insert automatic renewal successor: %w", err)
	}
	if err := snapshotComboSquadsTx(ctx, tx, successorID, plan.Combo.IncludedSquads); err != nil {
		return err
	}
	for _, addon := range plan.Addons {
		if _, err := tx.ExecContext(ctx, `INSERT INTO purchase_addons(purchase_id,group_id,charged_txb_minor) VALUES(?,?,?)`, successorID, addon.GroupID, addon.PriceTXBMinor); err != nil {
			return fmt.Errorf("snapshot automatic renewal add-on: %w", err)
		}
	}
	return nil
}

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	rolloverpkg "github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

func (s *Store) retryAttempt_commitAutoRenewal(ctx context.Context, purchaseID string, excludedAddonIDs []string, rolloverCountsTowardBalance bool, now time.Time) (model.Purchase, error) {
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
	rollover, err := scanRollover(tx.QueryRowContext(ctx, rolloverSelect+` WHERE purchase_id=?`, purchaseID))
	if err != nil {
		return model.Purchase{}, err
	}
	calculated := rollover.Status == "calculated"
	resumingExpired := rollover.Status == "zero" && plan.Purchase.Status == "expired"
	if !calculated && !resumingExpired {
		return model.Purchase{}, ErrConflict
	}
	var balance int64
	if err := tx.QueryRowContext(ctx, `SELECT txb_minor FROM balances WHERE user_id=?`, userID).Scan(&balance); err != nil {
		return model.Purchase{}, fmt.Errorf("load automatic renewal balance: %w", err)
	}
	credit := int64(0)
	if calculated {
		credit = calculatedRolloverCredit(rollover)
	}
	if !renewalFundsCover(balance, credit, plan.NetMinor, rolloverCountsTowardBalance) {
		settled, failErr := s.failCalculatedAutoRenewalTx(ctx, tx, purchaseID, userID, AutoRenewalReasonInsufficientBalance, now)
		if failErr != nil {
			return model.Purchase{}, failErr
		}
		if !settled {
			return model.Purchase{}, ErrConflict
		}
		if err := tx.Commit(); err != nil {
			return model.Purchase{}, fmt.Errorf("commit insufficient automatic renewal: %w", err)
		}
		return model.Purchase{}, ErrInsufficientBalance
	}
	if credit > 0 {
		creditedBalance, creditErr := changeBalanceTx(ctx, tx, userID, credit, now)
		if creditErr != nil {
			return model.Purchase{}, creditErr
		}
		if _, err := insertLedgerTx(ctx, tx, userID, credit, creditedBalance, "rollover_credit", purchaseID, "unused traffic rollover", now); err != nil {
			return model.Purchase{}, err
		}
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
	if calculated {
		version := rolloverpkg.UsageAlgorithmVersion
		if rollover.AlgorithmVersion == rolloverpkg.NativeUsageAlgorithmVersion {
			version = rolloverpkg.NativeUsageAlgorithmVersion
		}
		result, updateErr := tx.ExecContext(ctx, `UPDATE purchase_rollovers SET status=?,eligible_unused_bytes=?,credited_txb_minor=?,algorithm_version=?,updated_at=?,completed_at=?
			WHERE purchase_id=? AND status='calculated'`, rolloverStatusForCredit(credit), normalizedRolloverEligible(rollover), credit,
			version, stamp(now), stamp(now), purchaseID)
		if updateErr != nil {
			return model.Purchase{}, fmt.Errorf("complete automatic rollover: %w", updateErr)
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return model.Purchase{}, rowsErr
		} else if affected != 1 {
			return model.Purchase{}, ErrConflict
		}
		result, updateErr = tx.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=? WHERE id=? AND status IN ('active','activating')`, stamp(now), purchaseID)
		if updateErr != nil {
			return model.Purchase{}, fmt.Errorf("expire automatic renewal source: %w", updateErr)
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return model.Purchase{}, rowsErr
		} else if affected != 1 {
			return model.Purchase{}, ErrConflict
		}
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
		reward_rollover_min_remaining_bps,reward_traffic_renewal,reward_renewal_traffic_limit_bytes,created_at,updated_at)
		VALUES(?,?,?,?,?,?,'activating',?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, successorID, userID, plan.Combo.ID, plan.NetMinor,
		stamp(plan.ScheduledAt), stamp(plan.NextCycleEndsAt), couponGrantID, plan.GrossMinor, coreGross, plan.DiscountMinor, 1,
		boolInt(plan.Purchase.RecurringDiscountAttached), purchaseID, "automatic-renewal:"+purchaseID, plan.trafficLimitOverride, plan.resetStrategyOverride,
		plan.squadUUIDsOverride, plan.rewardRenewalPrice, plan.rewardRolloverBPS, boolInt(plan.rewardTrafficRenewal),
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

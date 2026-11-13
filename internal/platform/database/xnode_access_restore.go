package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// RepairXNodeAccessPeriod restores an unsettled allowance or creates its missing initial period. The
// serializable transaction fences entitlement/operation changes and pointer races.
func (s *Store) RepairXNodeAccessPeriod(ctx context.Context, identity xnode.Identity, purchase model.Purchase, period xnode.Period, generation int64, at time.Time, create bool) error {
	for attempt := 0; ; attempt++ {
		err := s.restoreXNodeAccessPeriod(ctx, identity, purchase, period, generation, at, create)
		if !s.retryTransaction(ctx, err, attempt) {
			return err
		}
	}
}

func (s *Store) restoreXNodeAccessPeriod(ctx context.Context, identity xnode.Identity, purchase model.Purchase, period xnode.Period, generation int64, at time.Time, create bool) error {
	if identity.UserID == "" || identity.ServiceName != "" || purchase.UserID != identity.UserID || generation < 0 ||
		period.ID == "" || period.ProxyUserID != identity.ProxyUserID || period.PurchaseID != purchase.ID {
		return xnode.ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockNativeMemberIdentityTx(ctx, tx, identity.UserID, generation, true); err != nil {
		return err
	}
	if err = lockMemberPurchaseTx(ctx, tx, identity.UserID); err != nil {
		return err
	}
	blocked, err := comboControlConflictFrom(ctx, tx, identity.UserID)
	if err != nil {
		return err
	}
	if blocked {
		return xnode.ErrAccessOperationPending
	}
	current, err := activePurchaseAt(ctx, tx, identity.UserID, at)
	if errors.Is(err, ErrNotFound) {
		// Match EffectivePurchase's already-paid queued successor lineage.
		current, err = scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=$1 AND purchases.user_id=$2 AND purchases.status IN ('active','activating')`, purchase.ID, identity.UserID))
		if err == nil {
			var linked bool
			linked, err = purchasePeriodContinuityTx(ctx, tx, current, purchase.ValidUntil)
			if err == nil && (!linked || at.Before(current.ValidUntil) || !at.Before(purchase.ValidUntil)) {
				return xnode.ErrConflict
			}
		}
	}
	if errors.Is(err, ErrNotFound) {
		return xnode.ErrConflict
	}
	if err != nil {
		return err
	}
	if current.ID != purchase.ID || !current.UpdatedAt.Equal(purchase.UpdatedAt) || current.TrafficLimitBytes != purchase.TrafficLimitBytes || current.ResetStrategy != purchase.ResetStrategy {
		return xnode.ErrConflict
	}
	if period.Status != "open" && period.Status != "closing" {
		return xnode.ErrAccessAccountingPending
	}
	var eligible bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_identities i JOIN users u ON u.id=i.user_id WHERE i.proxy_user_id=$1 AND i.user_id=$2 AND COALESCE(i.current_period_id,'')=$3 AND i.access_disabled=0 AND u.onboarding_state='complete') AND NOT EXISTS(SELECT 1 FROM xnode_periods WHERE proxy_user_id=$4 AND purchase_id=$5 AND status='open' AND id<>$6)`, identity.ProxyUserID, identity.UserID, identity.CurrentPeriodID, identity.ProxyUserID, purchase.ID, period.ID).Scan(&eligible)
	if err != nil {
		return err
	}
	if !eligible {
		return xnode.ErrConflict
	}
	var result sql.Result
	if create {
		result, err = tx.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,purchase_id,started_at,limit_bytes,strategy,last_reset_at,status) SELECT $1,$2,$3,$4,$5,$6,$7,'open' WHERE NOT EXISTS(SELECT 1 FROM xnode_periods WHERE proxy_user_id=$8 AND purchase_id=$9)`, period.ID, identity.ProxyUserID, purchase.ID, stamp(at), purchase.TrafficLimitBytes, purchase.ResetStrategy, stamp(at), identity.ProxyUserID, purchase.ID)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE xnode_periods SET status='open',ended_at=NULL WHERE id=$1 AND proxy_user_id=$2 AND purchase_id=$3 AND status=$4 AND closure_generation=$5`, period.ID, identity.ProxyUserID, purchase.ID, period.Status, period.ClosureGeneration)
	}
	if err != nil {
		return err
	}
	if err = requireAccessRow(result); err != nil {
		return err
	}
	if period.Status == "closing" {
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_period_nodes p SET flushed_sequence=NULL,flushed_stream_id='',flushed_legacy_sequence=0
			WHERE p.period_id=$1 AND NOT EXISTS(SELECT 1 FROM xnode_nodes n WHERE n.id=p.node_id AND n.enabled=0
			AND (n.removing_at IS NOT NULL OR n.retired_at IS NOT NULL) AND p.flushed_sequence IS NOT NULL)`, period.ID); err != nil {
			return err
		}
	}
	result, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=$1 WHERE proxy_user_id=$2 AND COALESCE(current_period_id,'')=$3`, period.ID, identity.ProxyUserID, identity.CurrentPeriodID)
	if err != nil {
		return err
	}
	if err = requireAccessRow(result); err != nil {
		return err
	}
	return tx.Commit()
}

func requireAccessRow(result sql.Result) error {
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return xnode.ErrConflict
	}
	return err
}

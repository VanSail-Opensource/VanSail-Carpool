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
func (s *Store) RepairXNodeAccessPeriod(ctx context.Context, identity xnode.Identity, purchase model.Purchase, period xnode.Period, at time.Time, create bool) error {
	for attempt := 0; ; attempt++ {
		err := s.restoreXNodeAccessPeriod(ctx, identity, purchase, period, at, create)
		if !s.retryTransaction(ctx, err, attempt) {
			return err
		}
	}
}

func (s *Store) restoreXNodeAccessPeriod(ctx context.Context, identity xnode.Identity, purchase model.Purchase, period xnode.Period, at time.Time, create bool) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	blocked, err := comboControlConflictFrom(ctx, tx, identity.UserID)
	if err != nil {
		return err
	}
	if blocked {
		return xnode.ErrAccessOperationPending
	}
	current, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.user_id=? AND purchases.status IN ('active','activating') AND purchases.valid_from<=? AND purchases.valid_until>? ORDER BY purchases.valid_from DESC LIMIT 1`, identity.UserID, stamp(at), stamp(at)))
	if errors.Is(err, ErrNotFound) {
		// Match EffectivePurchase's already-paid queued successor lineage.
		current, err = scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=? AND purchases.user_id=? AND purchases.status IN ('active','activating') AND EXISTS(SELECT 1 FROM purchases next WHERE next.user_id=purchases.user_id AND next.status='queued' AND next.valid_from=purchases.valid_until AND next.valid_from<=? AND next.valid_until=? AND next.valid_until>?)`, purchase.ID, identity.UserID, stamp(at), stamp(purchase.ValidUntil), stamp(at)))
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
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_identities i JOIN users u ON u.id=i.user_id WHERE i.proxy_user_id=? AND i.user_id=? AND COALESCE(i.current_period_id,'')=? AND i.access_disabled=0 AND u.onboarding_state='complete') AND NOT EXISTS(SELECT 1 FROM xnode_periods WHERE proxy_user_id=? AND purchase_id=? AND status='open' AND id<>?)`, identity.ProxyUserID, identity.UserID, identity.CurrentPeriodID, identity.ProxyUserID, purchase.ID, period.ID).Scan(&eligible)
	if err != nil {
		return err
	}
	if !eligible {
		return xnode.ErrConflict
	}
	var result sql.Result
	if create {
		result, err = tx.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,purchase_id,started_at,limit_bytes,strategy,last_reset_at,status) SELECT ?,?,?,?,?,?,?,'open' WHERE NOT EXISTS(SELECT 1 FROM xnode_periods WHERE proxy_user_id=? AND purchase_id=?)`, period.ID, identity.ProxyUserID, purchase.ID, stamp(at), purchase.TrafficLimitBytes, purchase.ResetStrategy, stamp(at), identity.ProxyUserID, purchase.ID)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE xnode_periods SET status='open',ended_at=NULL WHERE id=? AND proxy_user_id=? AND purchase_id=? AND status=?`, period.ID, identity.ProxyUserID, purchase.ID, period.Status)
	}
	if err != nil {
		return err
	}
	if err = requireAccessRow(result); err != nil {
		return err
	}
	if period.Status == "closing" {
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_period_nodes SET flushed_sequence=NULL WHERE period_id=?`, period.ID); err != nil {
			return err
		}
	}
	result, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=? WHERE proxy_user_id=? AND COALESCE(current_period_id,'')=?`, period.ID, identity.ProxyUserID, identity.CurrentPeriodID)
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

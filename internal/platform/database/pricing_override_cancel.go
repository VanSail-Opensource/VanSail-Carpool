package database

import (
	"context"
	"time"
)

// CancelPricingIntervention stops future intervention effects without restoring stale state.
func (s *Store) retryAttempt_CancelPricingIntervention(ctx context.Context, actor, id, reason string, now time.Time) error {
	if reason == "" || len(reason) > 1000 {
		return ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var serial int
	if err = tx.QueryRowContext(ctx, `SELECT singleton FROM pricing_worker_watermark WHERE singleton=1 /*pg:FOR UPDATE*/`).Scan(&serial); err != nil {
		return err
	}
	if err = pricingAdminTx(ctx, tx, actor); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE pricing_admin_overrides SET status='CANCELLED' WHERE id=? AND status='ACTIVE'`, id)
	if err != nil {
		return err
	}
	if n, e := result.RowsAffected(); e != nil {
		return e
	} else if n != 1 {
		return ErrConflict
	}
	if err = expirePricingOverridesTx(ctx, tx, now); err != nil {
		return err
	}
	if err = pricingAuditTx(ctx, tx, actor, "intervention.cancel", id, reason, nil, now); err != nil {
		return err
	}
	if err = pricingChangeNoticeTx(ctx, tx, actor, "CANCEL", id, reason, now); err != nil {
		return err
	}
	if err = tx.Commit(); err == nil {
		s.pricingCache.Clear()
		s.pricingExposureCache.Clear()
	}
	return err
}

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

// DueAutoRenewal identifies a currently enabled source term due for renewal.
type DueAutoRenewal struct {
	PurchaseID string
	UserID     string
}

// HasEnabledAutoRenewal reports whether an active entitlement chain blocks catalog checkout.
func (s *Store) HasEnabledAutoRenewal(ctx context.Context, userID string, now time.Time) (bool, error) {
	until, err := s.AutoRenewalBlockedUntil(ctx, userID, now)
	return until != nil, err
}

// SetAutoRenewal stores the member's explicit choice for an active, activating,
// or settled expired source term. Callers must validate eligibility first.
func (s *Store) retryAttempt_SetAutoRenewal(ctx context.Context, userID, purchaseID string, enabled bool, now time.Time) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin automatic renewal update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockMemberPurchaseTx(ctx, tx, userID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=$1,auto_renew_failure_reason=CASE WHEN $2=1 THEN '' ELSE auto_renew_failure_reason END,
		auto_renew_failed_at=CASE WHEN $3=1 THEN NULL ELSE auto_renew_failed_at END,updated_at=$4
		WHERE id=$5 AND user_id=$6 AND status IN ('active','activating','expired')`, boolInt(enabled), boolInt(enabled), boolInt(enabled), stamp(now), purchaseID, userID)
	if err != nil {
		return fmt.Errorf("set automatic renewal: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect automatic renewal update: %w", err)
	}
	if affected == 1 {
		if !enabled {
			if _, err := tx.ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=0,updated_at=$1
				WHERE auto_renew_source_purchase_id=$2 AND status='queued'`, stamp(now), purchaseID); err != nil {
				return fmt.Errorf("disable queued automatic renewal successor: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit automatic renewal update: %w", err)
		}
		return nil
	}
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT (EXISTS(SELECT 1 FROM purchases WHERE id=$1 AND user_id=$2))::integer", purchaseID, userID).Scan(&exists); err != nil {
		return fmt.Errorf("find automatic renewal purchase: %w", err)
	}
	if err := tx.Rollback(); err != nil {
		return fmt.Errorf("close automatic renewal update: %w", err)
	}
	if exists == 0 {
		return ErrNotFound
	}
	return ErrConflict
}

func (s *Store) DueAutoRenewals(ctx context.Context, now time.Time) ([]DueAutoRenewal, error) {
	_, upper := purchaseWindowCandidateBounds(now)
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,valid_until FROM purchases WHERE auto_renew_enabled=1 AND valid_until<=$1 AND status IN ('active','activating','expired')
		AND NOT EXISTS (SELECT 1 FROM purchases successor WHERE successor.auto_renew_source_purchase_id=purchases.id
			AND successor.status IN ('queued','activating','active'))`, upper)
	if err != nil {
		return nil, fmt.Errorf("list due automatic renewals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	type candidate struct {
		item  DueAutoRenewal
		until time.Time
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item DueAutoRenewal
		var rawUntil string
		if err := rows.Scan(&item.PurchaseID, &item.UserID, &rawUntil); err != nil {
			return nil, err
		}
		until, err := parseStamp(rawUntil)
		if err != nil {
			return nil, err
		}
		if !until.After(now) {
			candidates = append(candidates, candidate{item, until})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].until.Equal(candidates[j].until) {
			return candidates[i].item.PurchaseID < candidates[j].item.PurchaseID
		}
		return candidates[i].until.Before(candidates[j].until)
	})
	result := make([]DueAutoRenewal, 0, len(candidates))
	for _, item := range candidates {
		result = append(result, item.item)
	}
	return result, nil
}

// MarkAutoRenewalFailed disables a due source without charging its owner.
func (s *Store) retryAttempt_MarkAutoRenewalFailed(ctx context.Context, purchaseID, reason string, now time.Time) error {
	if reason == "" {
		return ErrConflict
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin automatic renewal failure: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var userID, rawUntil string
	err = tx.QueryRowContext(ctx, `SELECT user_id,valid_until FROM purchases WHERE id=$1`, purchaseID).Scan(&userID, &rawUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load automatic renewal failure owner: %w", err)
	}
	if err = lockMemberPurchaseTx(ctx, tx, userID); err != nil {
		return err
	}
	if _, found, err := automaticRenewalSuccessorIDTx(ctx, tx, purchaseID); err != nil {
		return err
	} else if found {
		if _, err = tx.ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=0,updated_at=$1 WHERE id=$2 AND auto_renew_enabled=1`, stamp(now), purchaseID); err != nil {
			return err
		}
		return tx.Commit()
	}
	// An older term can retain a stale preference after a later purchase has
	// already taken over. Retire that preference without reporting a failure.
	until, err := parseStamp(rawUntil)
	if err != nil {
		return err
	}
	superseded, err := otherRenewalPurchaseEndsAfter(ctx, tx, userID, purchaseID, until)
	if err != nil {
		return err
	}
	if superseded {
		if _, err = tx.ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=0,updated_at=$1 WHERE id=$2 AND auto_renew_enabled=1`, stamp(now), purchaseID); err != nil {
			return err
		}
		return tx.Commit()
	}
	if _, err := s.failAutoRenewalTx(ctx, tx, purchaseID, userID, reason, now.UTC()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit automatic renewal failure: %w", err)
	}
	return nil
}

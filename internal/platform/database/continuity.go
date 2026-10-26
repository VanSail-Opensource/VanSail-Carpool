package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

// Native compilation derives queued paid access directly from purchase state.
func enqueuePurchaseTransitionTx(ctx context.Context, tx *sql.Tx, purchaseID, status string, _ time.Time, now time.Time) error {
	if status == "activating" {
		return insertOutboxTx(ctx, tx, "xnode_apply_entitlement", `{"purchaseId":"`+purchaseID+`"}`, now, now)
	}
	return nil
}

// Retire only unclaimed legacy jobs. Existing claims complete through the
// compatibility handler and retain their generation fence and audit history.
func retirePurchaseContinuityTx(ctx context.Context, tx *sql.Tx, purchaseID string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='done',last_error='',updated_at=?
        WHERE kind=? AND json_extract(payload,'$.purchaseId')=? AND status IN ('pending','failed')`,
		stamp(now), jobpayload.ContinuityKind, purchaseID)
	return err
}

// RetireContinuityJobs acknowledges dormant pre-upgrade jobs without extending
// an entitlement, deleting history, or touching a currently owned claim.
func (s *Store) RetireContinuityJobs(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET status='done',last_error='',updated_at=?
        WHERE kind=? AND status IN ('pending','failed')`, stamp(now), jobpayload.ContinuityKind)
	return err
}

// ContinuityEntitlement returns the current term with the queued successor's
// lineage, so a retried provider extension cannot turn into an expiry gap.
func (s *Store) ContinuityEntitlement(ctx context.Context, successorID string, _ time.Time) (*model.Purchase, error) {
	var userID, status, validFromValue, validUntilValue string
	err := s.db.QueryRowContext(ctx, `SELECT user_id,status,valid_from,valid_until FROM purchases WHERE id=?`, successorID).
		Scan(&userID, &status, &validFromValue, &validUntilValue)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load continuity successor: %w", err)
	}
	validFrom, err := parseStamp(validFromValue)
	if err != nil {
		return nil, err
	}
	if status != "queued" {
		return nil, nil
	}
	current, err := scanPurchase(s.db.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.user_id=?
		AND purchases.status IN ('active','activating') AND purchases.valid_until=? ORDER BY purchases.valid_from DESC LIMIT 1`, userID, stamp(validFrom)))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	current.SquadUUIDs, err = s.purchaseSquads(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	current.ValidUntil, err = parseStamp(validUntilValue)
	return &current, err
}

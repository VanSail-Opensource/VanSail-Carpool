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

// EntitlementContinuityLead is the provider-expiry preparation window.
const EntitlementContinuityLead = 3 * time.Minute

func enqueuePurchaseTransitionTx(ctx context.Context, tx *sql.Tx, purchaseID, status string, validFrom, now time.Time) error {
	switch status {
	case "activating":
		return insertOutboxTx(ctx, tx, "remna_apply_entitlement", `{"purchaseId":"`+purchaseID+`"}`, now, now)
	case "queued":
		return insertOutboxTx(ctx, tx, jobpayload.ContinuityKind, `{"purchaseId":"`+purchaseID+`"}`, validFrom.Add(-EntitlementContinuityLead), now)
	default:
		return nil
	}
}

func reconcilePurchaseContinuityTx(ctx context.Context, tx *sql.Tx, purchaseID, status string, validFrom, now time.Time) error {
	var processing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=?
		AND json_extract(payload,'$.purchaseId')=? AND status='processing'`, jobpayload.ContinuityKind, purchaseID).Scan(&processing); err != nil {
		return err
	}
	if processing != 0 {
		return ErrConflict
	}
	if status != "queued" {
		_, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind=?
			AND json_extract(payload,'$.purchaseId')=? AND status IN ('pending','done','failed')`, jobpayload.ContinuityKind, purchaseID)
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind=?
		AND json_extract(payload,'$.purchaseId')=? AND status IN ('done','failed')`, jobpayload.ContinuityKind, purchaseID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET attempts=0,last_error='',available_at=?,updated_at=?
		WHERE kind=? AND json_extract(payload,'$.purchaseId')=? AND status='pending'`,
		stamp(validFrom.Add(-EntitlementContinuityLead)), stamp(now), jobpayload.ContinuityKind, purchaseID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 1 {
		return ErrConflict
	}
	if affected == 1 {
		return nil
	}
	return enqueuePurchaseTransitionTx(ctx, tx, purchaseID, status, validFrom, now)
}

// EnqueueContinuityBacklog repairs every queued term created before continuity
// jobs were introduced. Normal writes enqueue in the purchase transaction.
func (s *Store) EnqueueContinuityBacklog(ctx context.Context, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin continuity backlog: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT queued.id,queued.valid_from FROM purchases queued
		WHERE queued.status='queued' AND NOT EXISTS (
			SELECT 1 FROM outbox_jobs job WHERE job.kind=?
			AND json_extract(job.payload,'$.purchaseId')=queued.id)
		ORDER BY queued.valid_from,queued.id`, jobpayload.ContinuityKind)
	if err != nil {
		return fmt.Errorf("list continuity backlog: %w", err)
	}
	type pending struct{ id, validFrom string }
	items := make([]pending, 0)
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.id, &item.validFrom); err != nil {
			_ = rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, item := range items {
		validFrom, err := parseStamp(item.validFrom)
		if err != nil {
			return err
		}
		if err := enqueuePurchaseTransitionTx(ctx, tx, item.id, "queued", validFrom, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ContinuityEntitlement returns the current term with the queued successor's
// expiry. It remains available after the boundary until rollover changes the
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

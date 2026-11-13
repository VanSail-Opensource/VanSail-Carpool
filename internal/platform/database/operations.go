package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func insertOutboxTx(ctx context.Context, tx *sql.Tx, kind, payload string, availableAt, now time.Time) error {
	_, err := insertOutboxJobTx(ctx, tx, kind, payload, availableAt, now)
	return err
}

func insertOutboxJobTx(ctx context.Context, tx *sql.Tx, kind, payload string, availableAt, now time.Time) (string, error) {
	kind = strings.TrimSpace(kind)
	var typedPayload map[string]json.RawMessage
	if kind == "" || json.Unmarshal([]byte(payload), &typedPayload) != nil || len(typedPayload) == 0 {
		return "", errors.New("outbox kind and typed payload are required")
	}
	canonicalPayload, err := json.Marshal(typedPayload)
	if err != nil {
		return "", fmt.Errorf("canonicalize outbox payload: %w", err)
	}
	id, err := ids.New()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT   INTO outbox_jobs(id,kind,payload,status,attempts,available_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING `, id, kind, string(canonicalPayload), "pending", 0, stamp(availableAt), stamp(now), stamp(now))
	if err != nil {
		return "", fmt.Errorf("enqueue outbox job: %w", err)
	}
	return id, nil
}

// EnqueueOutbox appends a durable job.
func (s *Store) retryAttempt_EnqueueOutbox(ctx context.Context, kind, payload string, availableAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertOutboxTx(ctx, tx, kind, payload, availableAt, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

// EnqueueDueEntitlementTransitions turns time-bound state changes into durable work.
func (s *Store) retryAttempt_EnqueueDueEntitlementTransitions(ctx context.Context, now time.Time) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	expired, unopened, queued, err := dueEntitlementTransitionsTx(ctx, tx, now)
	if err != nil {
		return err
	}
	for _, purchaseID := range expired {
		if err := insertOutboxTx(ctx, tx, "purchase_expire", `{"purchaseId":"`+purchaseID+`"}`, now, now); err != nil {
			return err
		}
	}
	// Queued terms that were never activated have no upstream traffic to settle.
	for _, purchaseID := range unopened {
		if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=$1
			WHERE id=$2 AND status IN ('queued','failed')`, stamp(now), purchaseID); err != nil {
			return err
		}
	}
	for _, purchaseID := range queued {
		result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='activating',updated_at=$1 WHERE id=$2 AND status='queued'
			AND NOT EXISTS (SELECT 1 FROM purchases prior WHERE prior.user_id=purchases.user_id AND prior.status IN ('active','activating'))`, stamp(now), purchaseID)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			continue
		}
		if err := applyPendingExtensionsToActivationTx(ctx, tx, purchaseID, now); err != nil {
			return err
		}
		if err := insertOutboxTx(ctx, tx, "xnode_apply_entitlement", `{"purchaseId":"`+purchaseID+`"}`, now, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ExpirePurchase closes a delayed activation and requests a user-level entitlement reconciliation.
func (s *Store) retryAttempt_ExpirePurchase(ctx context.Context, purchaseID string, now time.Time) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var userID, status, rawUntil string
	if err := tx.QueryRowContext(ctx, `SELECT user_id,status,valid_until FROM purchases WHERE id=$1 FOR UPDATE`, purchaseID).Scan(&userID, &status, &rawUntil); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status == "expired" || status == "cancelled" {
		return nil
	}
	until, err := parseStamp(rawUntil)
	if err != nil {
		return err
	}
	// The worker may have read this term before an administrator extended it.
	if until.After(now) {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=$1 WHERE id=$2`, stamp(now), purchaseID); err != nil {
		return err
	}
	if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+userID+`"}`, now, now); err != nil {
		return err
	}
	if err := s.insertExpirationNotificationTx(ctx, tx, purchaseID, userSyncGate(userID), now); err != nil {
		return err
	}
	return tx.Commit()
}

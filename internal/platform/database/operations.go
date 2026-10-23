package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"strings"
	"time"
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
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO outbox_jobs(id,kind,payload,status,attempts,available_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, id, kind, string(canonicalPayload), "pending", 0, stamp(availableAt), stamp(now), stamp(now))
	if err != nil {
		return "", fmt.Errorf("enqueue outbox job: %w", err)
	}
	return id, nil
}

// EnqueueOutbox appends a durable job.
func (s *Store) EnqueueOutbox(ctx context.Context, kind, payload string, availableAt time.Time) error {
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
func (s *Store) EnqueueDueEntitlementTransitions(ctx context.Context, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	type expiredItem struct{ purchaseID, userID string }
	rows, err := tx.QueryContext(ctx, `SELECT id,user_id FROM purchases WHERE status IN ('active','activating') AND valid_until<=?`, stamp(now))
	if err != nil {
		return err
	}
	expired := make([]expiredItem, 0)
	for rows.Next() {
		var item expiredItem
		if err := rows.Scan(&item.purchaseID, &item.userID); err != nil {
			_ = rows.Close()
			return err
		}
		expired = append(expired, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, item := range expired {
		result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO purchase_rollovers(purchase_id,status,traffic_limit_bytes,minimum_remaining_bps,net_paid_txb_minor,created_at,updated_at)
			SELECT p.id,'pending',COALESCE(p.entitlement_traffic_limit_bytes,c.traffic_limit_bytes),
				COALESCE(p.reward_rollover_min_remaining_bps,c.rollover_min_remaining_bps),p.charged_txb_minor,?,?
			FROM purchases p JOIN combos c ON c.id=p.combo_id WHERE p.id=?`, stamp(now), stamp(now), item.purchaseID)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected == 1 {
			if err := insertOutboxTx(ctx, tx, "rollover_finalize", `{"purchaseId":"`+item.purchaseID+`"}`, now, now); err != nil {
				return err
			}
		}
	}
	// Queued terms that were never activated have no upstream traffic to settle.
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=? WHERE status IN ('queued','failed') AND valid_until<=?`, stamp(now), stamp(now)); err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT id FROM purchases WHERE status='queued' AND valid_from<=? AND valid_until>?`, stamp(now), stamp(now))
	if err != nil {
		return err
	}
	queued := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		queued = append(queued, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, purchaseID := range queued {
		result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='activating',updated_at=? WHERE id=? AND status='queued'
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
func (s *Store) ExpirePurchase(ctx context.Context, purchaseID string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var userID, status string
	if err := tx.QueryRowContext(ctx, `SELECT user_id,status FROM purchases WHERE id=?`, purchaseID).Scan(&userID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status == "expired" || status == "cancelled" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=? WHERE id=?`, stamp(now), purchaseID); err != nil {
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

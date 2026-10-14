package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// ApproveCompensationEvent rechecks frozen recipients and atomically queues exact-state sync.
func (s *Store) ApproveCompensationEvent(ctx context.Context, input compensation.ReviewInput, fingerprint string, now time.Time) (compensation.Event, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return compensation.Event{}, err
	}
	defer func() { _ = tx.Rollback() }()
	event, replay, err := loadReviewableCompensationEvent(ctx, tx, input, fingerprint)
	if replay {
		if err := tx.Commit(); err != nil {
			return compensation.Event{}, err
		}
		return s.compensationEventByID(ctx, input.EventID)
	}
	if err != nil || event.Status != "pending_review" {
		if err == nil {
			err = compensation.ErrConflict
		}
		return compensation.Event{}, err
	}
	if event.Revision != input.Revision {
		return compensation.Event{}, compensation.ErrConflict
	}
	targets, frozenCount, err := activeFrozenTargets(ctx, tx, input.EventID, now)
	if err != nil {
		return compensation.Event{}, err
	}
	if len(targets) == 0 {
		return compensation.Event{}, compensation.ErrConflict
	}
	items := make([]providerops.ItemInput, 0, len(targets))
	for _, target := range targets {
		items = append(items, providerops.ItemInput{Key: target.UserID, TargetType: "user", TargetID: target.UserID})
	}
	operation, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: input.ActorUserID,
		Kind: providerops.KindNodeCompensation, IdempotencyKey: input.IdempotencyKey, RequestFingerprint: fingerprint, Items: items}, now)
	if err != nil || replayed {
		if err == nil {
			err = compensation.ErrConflict
		}
		return compensation.Event{}, err
	}
	if err := applyCompensationExtension(ctx, s, tx, event, operation.Receipt.ID, targets, input.ExtensionMinutes, input.Reason, now); err != nil {
		return compensation.Event{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE node_compensation_events SET status='queued',final_extension_minutes=?,
		eligible_recipient_count=?,skipped_recipient_count=?,reviewed_by=?,reviewed_at=?,review_reason=?,review_idempotency_key=?,
		review_fingerprint=?,provider_operation_id=?,revision=revision+1,updated_at=? WHERE id=? AND status='pending_review' AND revision=?`,
		input.ExtensionMinutes, len(targets), frozenCount-len(targets), input.ActorUserID, stamp(now), strings.TrimSpace(input.Reason),
		input.IdempotencyKey, fingerprint, operation.Receipt.ID, stamp(now), input.EventID, input.Revision)
	if err != nil {
		return compensation.Event{}, mapCompensationReviewError(err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return compensation.Event{}, compensation.ErrConflict
	}
	if err := auditCompensationReview(ctx, tx, input, operation.Receipt.ID, len(targets), frozenCount-len(targets), now); err != nil {
		return compensation.Event{}, err
	}
	if err := tx.Commit(); err != nil {
		return compensation.Event{}, err
	}
	return s.compensationEventByID(ctx, input.EventID)
}

func loadReviewableCompensationEvent(ctx context.Context, tx *sql.Tx, input compensation.ReviewInput, fingerprint string) (compensation.Event, bool, error) {
	event, err := scanCompensationEvent(tx.QueryRowContext(ctx, compensationEventSelect+` WHERE id=?`, input.EventID))
	if err != nil {
		return event, false, err
	}
	var actor, key, storedFingerprint sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT reviewed_by,review_idempotency_key,review_fingerprint FROM node_compensation_events WHERE id=?`,
		input.EventID).Scan(&actor, &key, &storedFingerprint); err != nil {
		return event, false, err
	}
	if key.Valid {
		if actor.String == input.ActorUserID && key.String == input.IdempotencyKey && storedFingerprint.String == fingerprint {
			return event, true, nil
		}
		return event, false, compensation.ErrConflict
	}
	return event, false, nil
}

func activeFrozenTargets(ctx context.Context, tx *sql.Tx, eventID string, now time.Time) ([]adminBulkTarget, int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT purchases.id,purchases.user_id FROM node_compensation_event_recipients recipients
		JOIN purchases ON purchases.user_id=recipients.user_id WHERE recipients.event_id=? AND purchases.status='active'
		AND purchases.valid_from<=? AND purchases.valid_until>? ORDER BY purchases.user_id,purchases.valid_until,purchases.id`,
		eventID, stamp(now), stamp(now))
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	seen := map[string]struct{}{}
	targets := []adminBulkTarget{}
	for rows.Next() {
		var target adminBulkTarget
		if err := rows.Scan(&target.PurchaseID, &target.UserID); err != nil {
			return nil, 0, err
		}
		if _, ok := seen[target.UserID]; !ok {
			seen[target.UserID] = struct{}{}
			targets = append(targets, target)
		}
	}
	var count int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_compensation_event_recipients WHERE event_id=?`, eventID).Scan(&count)
	return targets, count, errors.Join(rows.Err(), err)
}

func auditCompensationReview(ctx context.Context, tx *sql.Tx, input compensation.ReviewInput, operationID string, eligible, skipped int, now time.Time) error {
	auditID, err := ids.New()
	if err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]any{"reason": input.Reason, "extensionMinutes": input.ExtensionMinutes,
		"eligibleRecipients": eligible, "skippedRecipients": skipped, "operationId": operationID})
	return insertAuditTx(ctx, tx, auditID, &input.ActorUserID, "node_compensation.approve", "node_compensation_event", input.EventID, string(detail), now)
}

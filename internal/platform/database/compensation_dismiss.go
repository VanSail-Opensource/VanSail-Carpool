package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// DismissCompensationEvent records one idempotent reviewed rejection.
func (s *Store) retryAttempt_DismissCompensationEvent(ctx context.Context, input compensation.ReviewInput, fingerprint string, now time.Time) (compensation.Event, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
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
	if err != nil {
		return compensation.Event{}, err
	}
	if event.Status != "pending_review" || event.Revision != input.Revision {
		return compensation.Event{}, compensation.ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE node_compensation_events SET status='dismissed',reviewed_by=?,reviewed_at=?,
		review_reason=?,review_idempotency_key=?,review_fingerprint=?,eligible_recipient_count=0,
		skipped_recipient_count=frozen_recipient_count,revision=revision+1,updated_at=?
		WHERE id=? AND status='pending_review' AND revision=?`, input.ActorUserID, stamp(now), strings.TrimSpace(input.Reason),
		input.IdempotencyKey, fingerprint, stamp(now), input.EventID, input.Revision)
	if err != nil {
		return compensation.Event{}, mapCompensationReviewError(err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return compensation.Event{}, compensation.ErrConflict
	}
	if err := auditCompensationDismissal(ctx, tx, input, now); err != nil {
		return compensation.Event{}, err
	}
	if err := tx.Commit(); err != nil {
		return compensation.Event{}, err
	}
	return s.compensationEventByID(ctx, input.EventID)
}

func auditCompensationDismissal(ctx context.Context, tx *sql.Tx, input compensation.ReviewInput, now time.Time) error {
	auditID, err := ids.New()
	if err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]any{"reason": strings.TrimSpace(input.Reason)})
	return insertAuditTx(ctx, tx, auditID, &input.ActorUserID, "node_compensation.dismiss", "node_compensation_event", input.EventID, string(detail), now)
}

func mapCompensationReviewError(err error) error {
	if isUniqueViolation(err) {
		return compensation.ErrConflict
	}
	return err
}

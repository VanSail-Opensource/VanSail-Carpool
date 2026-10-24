package database

import (
	"context"
	"encoding/json"
	"time"
)

// RequeuePMItem is used only after a proven non-delivery (preflight or Telegram429).
func (s *Store) RequeuePMItem(ctx context.Context, operationID, key string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE provider_operation_items SET status='queued',attempt_started_at=NULL,error_code='',updated_at=?
		WHERE operation_id=? AND item_key=? AND status='processing' AND operation_id IN
		(SELECT id FROM provider_operations WHERE kind LIKE 'telegram_pm_%' AND status='processing')`, stamp(now), operationID, key)
	return err
}

// ResetMissingPMTopic accepts only the worker's definitive missing-topic response.
func (s *Store) retryAttempt_ResetMissingPMTopic(ctx context.Context, id, operationID string, missingTopicID int64, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE telegram_pm_conversations SET topic_id=NULL,profile_message_id=NULL,profile_state='missing',
		topic_state='new',topic_operation_id=NULL,updated_at=? WHERE id=? AND topic_id=?`, stamp(now), id, missingTopicID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='queued',attempt_started_at=NULL,
		completed_at=NULL,error_code='',provider_reference='',result_json='{}',updated_at=?
		WHERE operation_id=? AND operation_id IN (SELECT id FROM provider_operations WHERE kind LIKE 'telegram_pm_%' AND status='processing')`, stamp(now), operationID); err != nil {
		return err
	}
	return tx.Commit()
}

// SavePMTopicRepair binds only a topic/profile pair successfully probed by the queued executor.
func (s *Store) retryAttempt_SavePMTopicRepair(ctx context.Context, id, operationID string, topicID, profileID int64, now time.Time) error {
	if topicID <= 1 || profileID <= 0 {
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
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE telegram_pm_conversations SET topic_id=?,profile_message_id=?,profile_state='ready',topic_state='ready',
		topic_operation_id=?,updated_at=? WHERE id=? AND topic_state IN ('ready','pending_review')`, topicID, profileID, operationID, stamp(now), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrConflict
	}
	// Only waiters whose relay phase never started can resume. Ambiguous copies remain untouched.
	rows, err := tx.QueryContext(ctx, `SELECT o.id FROM provider_operations o JOIN provider_operation_items p ON p.operation_id=o.id AND p.item_key='topic'
		JOIN provider_operation_items r ON r.operation_id=o.id AND r.item_key='relay' AND r.status='queued' AND r.attempt_started_at IS NULL
		WHERE p.target_id=? AND o.kind='telegram_pm_relay' AND o.status='pending_review' AND o.error_code IN ('PM_TOPIC_UNCERTAIN','PM_PROFILE_UNCERTAIN')`, id)
	if err != nil {
		return err
	}
	var waiters []string
	for rows.Next() {
		var waiter string
		if err := rows.Scan(&waiter); err != nil {
			_ = rows.Close()
			return err
		}
		waiters = append(waiters, waiter)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, waiter := range waiters {
		if _, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='queued',attempt_started_at=NULL,completed_at=NULL,error_code='',updated_at=? WHERE operation_id=? AND item_key IN ('topic','profile')`, stamp(now), waiter); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE provider_operations SET status='queued',attempt_started_at=NULL,completed_at=NULL,error_code='',updated_at=? WHERE id=?`, stamp(now), waiter); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]string{"operationId": waiter})
		if err := insertOutboxTx(ctx, tx, "provider_operation", string(payload), now, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

package database

import (
	"context"
	"fmt"
	"time"
)

const (
	questionnaireImportRetention = 7 * 24 * time.Hour
	completedOutboxRetention     = 7 * 24 * time.Hour
	groupMessageEventRetention   = 31 * 24 * time.Hour
	groupMessageWindowRetention  = 365 * 24 * time.Hour
)

// PruneTransientRecords bounds retry artifacts and Telegram message-deduplication
// state. Accounting records, active jobs, and imports still eligible for retry
// are deliberately preserved.
func (s *Store) retryAttempt_PruneTransientRecords(ctx context.Context, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transient retention: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now = now.UTC()
	importCutoff := stamp(now.Add(-questionnaireImportRetention))

	// A failed settlement leaves its questionnaire in settling state while an
	// administrator may retry it. Once that bounded retry window expires, close
	// the questionnaire before erasing the upload and its failed job.
	if _, err := tx.ExecContext(ctx, `UPDATE questionnaires SET status='closed',updated_at=?
		WHERE status='settling' AND id IN (SELECT questionnaire_id FROM questionnaire_imports WHERE status='failed' AND updated_at<?)`, stamp(now), importCutoff); err != nil {
		return fmt.Errorf("close abandoned questionnaire settlements: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind='questionnaire_settlement' AND status<>'processing'
		AND json_extract(payload,'$.importId') IN (SELECT id FROM questionnaire_imports WHERE status IN ('preview','failed') AND updated_at<?)`, importCutoff); err != nil {
		return fmt.Errorf("prune abandoned questionnaire jobs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM questionnaire_imports WHERE status IN ('preview','failed') AND updated_at<?`, importCutoff); err != nil {
		return fmt.Errorf("prune abandoned questionnaire imports: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE status='done' AND updated_at<?`, stamp(now.Add(-completedOutboxRetention))); err != nil {
		return fmt.Errorf("prune completed outbox jobs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM activity_group_message_events WHERE created_at<?`, stamp(now.Add(-groupMessageEventRetention))); err != nil {
		return fmt.Errorf("prune group-message events: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM activity_group_message_windows WHERE updated_at<?`, stamp(now.Add(-groupMessageWindowRetention))); err != nil {
		return fmt.Errorf("prune group-message windows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transient retention: %w", err)
	}
	return nil
}

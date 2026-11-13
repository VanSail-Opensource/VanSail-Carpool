package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func pruneHousekeepingTx(ctx context.Context, tx *sql.Tx, cutoff7Days, cutoff24Hours, now time.Time,
	counts map[string]int64) error {
	var err error
	if counts["user_notification_events"], err = deleteCount(ctx, tx, `DELETE FROM user_notification_events
		WHERE queued_at IS NOT NULL AND queued_at<$1 AND `+purchaseTimestampOrderSQL("queued_at")+`<$2`, retentionCandidateUpperBound(now.Add(-3*24*time.Hour)), purchaseTimestampOrder(now.Add(-3*24*time.Hour))); err != nil {
		return fmt.Errorf("prune completed user notifications: %w", err)
	}
	if err := pruneAuthTokensTx(ctx, tx, now, counts); err != nil {
		return err
	}
	if counts["database_admin_reviews"], err = deleteCount(ctx, tx, `DELETE FROM database_admin_reviews WHERE consumed_at IS NOT NULL`); err != nil {
		return fmt.Errorf("prune expired database reviews: %w", err)
	}
	reviews, err := expiredTimestampIDsTx(ctx, tx, `SELECT id,expires_at FROM database_admin_reviews WHERE expires_at<$1`, now)
	if err != nil {
		return err
	}
	count, err := pruneTimestampIDsTx(ctx, tx, `DELETE FROM database_admin_reviews WHERE id=ANY($1)`, reviews)
	if err != nil {
		return err
	}
	counts["database_admin_reviews"] += count
	if counts["panel_entry_verification"], err = deleteCount(ctx, tx, `DELETE FROM panel_entry_verification WHERE NOT EXISTS(SELECT 1 FROM users WHERE users.telegram_id=panel_entry_verification.telegram_id)
 OR EXISTS(SELECT 1 FROM users WHERE users.telegram_id=panel_entry_verification.telegram_id AND (policy_accepted_at IS NOT NULL OR accepted_agreement_revision>0))`); err != nil {
		return fmt.Errorf("prune redundant panel entry verification: %w", err)
	}
	// Account deletion belongs exclusively to verified seven-day cleanup.
	// Incomplete onboarding and elapsed age are never deletion authority.
	counts["abandoned_users"] = 0
	if counts["maintenance_runs"], err = deleteCount(ctx, tx, `DELETE FROM maintenance_runs WHERE started_at<$1 AND `+purchaseTimestampOrderSQL("started_at")+`<$2`,
		retentionCandidateUpperBound(cutoff7Days), purchaseTimestampOrder(cutoff7Days)); err != nil {
		return fmt.Errorf("prune maintenance runs: %w", err)
	}
	return nil
}

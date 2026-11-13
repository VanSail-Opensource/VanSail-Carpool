package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const pmReceiptRetention = 30 * 24 * time.Hour

// Uncertainty remains in routing state after bounded attempt receipts are pruned.
func reconcileExpiredPMOperationsTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	var err error
	if counts["telegram_pm_payload_expiry"], err = deleteCount(ctx, tx, `UPDATE telegram_pm_payloads SET encrypted_payload='' WHERE expires_at<=$1 AND encrypted_payload<>''`, now.Unix()); err != nil {
		return err
	}
	if counts["telegram_pm_payload_cleanup"], err = deleteCount(ctx, tx, `UPDATE telegram_pm_payloads SET encrypted_payload='' WHERE encrypted_payload<>'' AND operation_id IN(SELECT operation_id FROM telegram_pm_payloads JOIN provider_operations operation ON operation.id=operation_id WHERE operation.status IN ('succeeded','failed','compensated','partial'))`); err != nil {
		return err
	}
	if counts["telegram_pm_topic_reviews"], err = deleteCount(ctx, tx, `UPDATE telegram_pm_conversations SET topic_state='pending_review',updated_at=$1
		WHERE topic_state='creating' AND topic_operation_id IN (SELECT id FROM maintenance_operation_candidates)`, stamp(now)); err != nil {
		return fmt.Errorf("retain uncertain PM topic state: %w", err)
	}
	if counts["telegram_pm_profile_reviews"], err = deleteCount(ctx, tx, `UPDATE telegram_pm_conversations SET profile_state='pending_review',updated_at=$1
		WHERE profile_message_id IS NULL AND profile_state<>'pending_review' AND EXISTS(
		SELECT 1 FROM provider_operation_items item WHERE item.operation_id IN (SELECT id FROM maintenance_operation_candidates)
		AND item.status='processing' AND ((item.target_type='pm_conversation' AND item.target_id=telegram_pm_conversations.id AND item.item_key='profile')
		OR (item.target_type='pm_topic_recovery' AND item.target_id ILIKE telegram_pm_conversations.id||':%')))`, stamp(now)); err != nil {
		return fmt.Errorf("retain uncertain PM profile state: %w", err)
	}
	cutoff := now.Add(-pmReceiptRetention)
	if counts["telegram_pm_updates"], err = deleteCount(ctx, tx, `DELETE FROM telegram_pm_updates WHERE created_at<$1 AND `+purchaseTimestampOrderSQL("created_at")+`<=$2`, retentionCandidateUpperBound(cutoff), purchaseTimestampOrder(cutoff)); err != nil {
		return fmt.Errorf("prune PM update receipts: %w", err)
	}
	return nil
}

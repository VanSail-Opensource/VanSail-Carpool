package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ResolvePaymentRefundOperation closes an interrupted refund from an authoritative callback.
func (s *Store) retryAttempt_ResolvePaymentRefundOperation(ctx context.Context, orderID, providerReference string, now time.Time) error {
	result, err := json.Marshal(map[string]string{"paymentOrderId": orderID, "source": "provider_callback"})
	if err != nil {
		return err
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin payment refund resolution: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	args := []any{providerReference, providerReference, string(result), stamp(now), stamp(now), orderID}
	_, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='succeeded',
		provider_reference=CASE WHEN ?<>'' THEN ? ELSE provider_reference END,error_code='',result_json=?,
		completed_at=?,updated_at=? WHERE target_type='payment_order' AND target_id=?
		AND status IN ('queued','processing','pending_review') AND operation_id IN (
			SELECT id FROM provider_operations WHERE kind='payment_refund'
			AND status IN ('queued','processing','pending_review'))`, args...)
	if err != nil {
		return fmt.Errorf("resolve payment refund item: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status='succeeded',
		provider_reference=CASE WHEN ?<>'' THEN ? ELSE provider_reference END,error_code='',result_json=?,
		completed_at=?,updated_at=? WHERE kind='payment_refund' AND id IN (
			SELECT operation_id FROM provider_operation_items WHERE target_type='payment_order' AND target_id=?)
		AND status IN ('queued','processing','pending_review')`, args...)
	if err != nil {
		return fmt.Errorf("resolve payment refund receipt: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit payment refund resolution: %w", err)
	}
	return nil
}

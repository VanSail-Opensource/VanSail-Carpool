package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ResolvePaymentCreateOperation lets an authoritative paid callback close an
// interrupted or review-required checkout command without another provider call.
func (s *Store) ResolvePaymentCreateOperation(ctx context.Context, orderID, providerReference string, now time.Time) error {
	result, err := json.Marshal(map[string]string{"paymentOrderId": orderID, "source": "provider_callback"})
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin payment operation resolution: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := resolvePaymentCreateOperationTx(ctx, tx, orderID, providerReference, string(result), now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit payment operation resolution: %w", err)
	}
	return nil
}

func resolvePaymentCreateOperationTx(ctx context.Context, tx *sql.Tx, orderID, providerReference, result string, now time.Time) error {
	args := []any{providerReference, providerReference, result, stamp(now), stamp(now), orderID}
	_, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='succeeded',
		provider_reference=CASE WHEN ?<>'' THEN ? ELSE provider_reference END,error_code='',result_json=?,
		completed_at=?,updated_at=? WHERE target_type='payment_order' AND target_id=?
		AND operation_id IN (SELECT id FROM provider_operations WHERE kind='payment_create')`, args...)
	if err != nil {
		return fmt.Errorf("resolve payment operation item: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status='succeeded',
		provider_reference=CASE WHEN ?<>'' THEN ? ELSE provider_reference END,error_code='',result_json=?,
		completed_at=?,updated_at=? WHERE kind='payment_create' AND id IN (
			SELECT operation_id FROM provider_operation_items WHERE target_type='payment_order' AND target_id=?)`, args...)
	if err != nil {
		return fmt.Errorf("resolve payment operation receipt: %w", err)
	}
	return nil
}

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// CompleteProviderOperationItemAndReleaseNotifications records a real worker
// success and releases only that item's provider-gated user messages.
func (s *Store) retryAttempt_CompleteProviderOperationItemAndReleaseNotifications(ctx context.Context, operationID, itemKey string,
	completion providerops.Completion, now time.Time) (providerops.Item, error) {
	if completion.Status != providerops.StatusSucceeded {
		return providerops.Item{}, ErrConflict
	}
	resultJSON, err := providerops.ResultObject(completion.ResultJSON)
	if err != nil {
		return providerops.Item{}, err
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Item{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status=?,provider_reference=?,error_code=?,
		result_json=?,completed_at=?,updated_at=? WHERE operation_id=? AND item_key=? AND status='processing'
		AND attempt_started_at IS NOT NULL`, completion.Status, completion.ProviderReference, operationCode(completion.ErrorCode),
		resultJSON, stamp(now), stamp(now), operationID, itemKey)
	if err != nil {
		return providerops.Item{}, fmt.Errorf("complete provider notification item: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return providerops.Item{}, rowsErr
		}
		return providerops.Item{}, ErrConflict
	}
	if _, err := s.releaseNotificationGateTx(ctx, tx, providerItemGate(operationID, itemKey), now); err != nil {
		return providerops.Item{}, err
	}
	item, err := scanProviderOperationItem(tx.QueryRowContext(ctx, providerOperationItemSelect+
		` WHERE operation_id=? AND item_key=?`, operationID, itemKey))
	if err != nil {
		return providerops.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return providerops.Item{}, err
	}
	return item, nil
}

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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Item{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status=$1,provider_reference=$2,error_code=$3,
		result_json=$4,completed_at=$5,updated_at=$6 WHERE operation_id=$7 AND item_key=$8 AND status='processing'
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
		` WHERE operation_id=$1 AND item_key=$2`, operationID, itemKey))
	if err != nil {
		return providerops.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return providerops.Item{}, err
	}
	return item, nil
}

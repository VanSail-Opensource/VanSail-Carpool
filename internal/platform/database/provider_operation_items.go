package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

const providerOperationItemSelect = `SELECT operation_id,item_key,target_type,target_id,status,provider_reference,
	error_code,result_json,attempt_started_at,completed_at,created_at,updated_at FROM provider_operation_items`

// ProviderOperationItems returns the bounded targets required by a worker.
func (s *Store) ProviderOperationItems(ctx context.Context, operationID string) ([]providerops.Item, error) {
	rows, err := s.db.QueryContext(ctx, providerOperationItemSelect+
		` WHERE operation_id=? ORDER BY item_key`, operationID)
	if err != nil {
		return nil, fmt.Errorf("list provider operation items: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]providerops.Item, 0)
	for rows.Next() {
		item, scanErr := scanProviderOperationItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListProviderOperationItems is a descriptive compatibility alias.
func (s *Store) ListProviderOperationItems(ctx context.Context, operationID string) ([]providerops.Item, error) {
	return s.ProviderOperationItems(ctx, operationID)
}

// BeginProviderOperationItemAttempt records intent before mutating one target.
func (s *Store) BeginProviderOperationItemAttempt(ctx context.Context, operationID, itemKey string, now time.Time) (providerops.Item, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE provider_operation_items SET status='processing',attempt_started_at=?,
		updated_at=? WHERE operation_id=? AND item_key=? AND status='queued'`, stamp(now), stamp(now), operationID, itemKey)
	if err != nil {
		return providerops.Item{}, fmt.Errorf("begin provider operation item: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return providerops.Item{}, rowsErr
	} else if affected != 1 {
		return providerops.Item{}, ErrConflict
	}
	return scanProviderOperationItem(s.db.QueryRowContext(ctx, providerOperationItemSelect+
		` WHERE operation_id=? AND item_key=?`, operationID, itemKey))
}

// CompleteProviderOperationItem stores one sanitized item result.
func (s *Store) CompleteProviderOperationItem(ctx context.Context, operationID, itemKey string, completion providerops.Completion, now time.Time) (providerops.Item, error) {
	if completion.Status != providerops.StatusSucceeded && completion.Status != providerops.StatusFailed &&
		completion.Status != providerops.StatusCompensated && completion.Status != providerops.StatusPendingReview {
		return providerops.Item{}, ErrConflict
	}
	resultJSON, err := providerops.ResultObject(completion.ResultJSON)
	if err != nil {
		return providerops.Item{}, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE provider_operation_items SET status=?,provider_reference=?,error_code=?,
		result_json=?,completed_at=?,updated_at=? WHERE operation_id=? AND item_key=? AND status='processing'
		AND attempt_started_at IS NOT NULL`, completion.Status, completion.ProviderReference, operationCode(completion.ErrorCode),
		resultJSON, stamp(now), stamp(now), operationID, itemKey)
	if err != nil {
		return providerops.Item{}, fmt.Errorf("complete provider operation item: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return providerops.Item{}, rowsErr
	} else if affected != 1 {
		return providerops.Item{}, ErrConflict
	}
	return scanProviderOperationItem(s.db.QueryRowContext(ctx, providerOperationItemSelect+
		` WHERE operation_id=? AND item_key=?`, operationID, itemKey))
}

func scanProviderOperationItem(row rowScanner) (providerops.Item, error) {
	var item providerops.Item
	var status, created, updated string
	var attemptNull, completedNull sql.NullString
	err := row.Scan(&item.OperationID, &item.Key, &item.TargetType, &item.TargetID, &status,
		&item.ProviderReference, &item.ErrorCode, &item.ResultJSON, &attemptNull, &completedNull, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return providerops.Item{}, ErrNotFound
	}
	if err != nil {
		return providerops.Item{}, fmt.Errorf("scan provider operation item: %w", err)
	}
	item.Status = providerops.Status(status)
	item.AttemptStartedAt, err = parseOptionalStamp(attemptNull)
	if err == nil {
		item.CompletedAt, err = parseOptionalStamp(completedNull)
	}
	if err == nil {
		item.CreatedAt, err = parseStamp(created)
	}
	if err == nil {
		item.UpdatedAt, err = parseStamp(updated)
	}
	return item, err
}

package database

import (
	"context"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ListAdminOperationsForUser includes owned and bulk-item operations.
func (s *Store) ListAdminOperationsForUser(ctx context.Context, userID string, limit int) ([]model.OperationReceipt, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, providerOperationSelect+` WHERE status IN ('queued','processing','pending_review','partial') AND (owner_user_id=? OR EXISTS (
		SELECT 1 FROM provider_operation_items WHERE operation_id=provider_operations.id
		AND target_type='user' AND target_id=?)) ORDER BY created_at DESC,id DESC LIMIT ?`, userID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list admin user operations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]model.OperationReceipt, 0)
	for rows.Next() {
		operation, scanErr := scanProviderOperation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, operation.Receipt)
	}
	return result, rows.Err()
}

package database

import (
	"context"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ListAdminPurchasesPage returns a filtered stable page for the entitlement inventory.
func (s *Store) ListAdminPurchasesPage(ctx context.Context, cursor, search, status string, limit int) ([]model.Purchase, *string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := purchaseSelect + ` WHERE 1=1`
	args := make([]any, 0, 8)
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		query += ` AND (purchases.id LIKE ? ESCAPE '\' COLLATE NOCASE
			OR purchases.user_id LIKE ? ESCAPE '\' COLLATE NOCASE
			OR purchases.combo_id LIKE ? ESCAPE '\' COLLATE NOCASE
			OR combos.name LIKE ? ESCAPE '\' COLLATE NOCASE)`
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if status != "" {
		query += ` AND purchases.status=?`
		args = append(args, status)
	}
	filter := pageFilterFingerprint(search, status)
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return nil, nil, err
		}
		query += ` AND (purchases.created_at<? OR (purchases.created_at=? AND purchases.id<?))`
		args = append(args, decoded.Timestamp, decoded.Timestamp, decoded.ID)
	}
	query += ` ORDER BY purchases.created_at DESC,purchases.id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list admin entitlement page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]model.Purchase, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanPurchase(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	for index := range items {
		items[index].SquadUUIDs, err = s.purchaseSquads(ctx, items[index].ID)
		if err != nil {
			return nil, nil, err
		}
	}
	if len(items) <= limit {
		return items, nil, nil
	}
	items = items[:limit]
	last := items[len(items)-1]
	next, err := encodeTimestampCursor(last.CreatedAt, last.ID, filter)
	return items, &next, err
}

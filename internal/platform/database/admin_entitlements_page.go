package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ListAdminPurchasesPage returns a filtered stable page for the entitlement inventory.
func (s *Store) ListAdminPurchasesPage(ctx context.Context, cursor, search, status string, limit int) ([]model.Purchase, *string, error) {
	return s.listAdminPurchasesPageAt(ctx, cursor, search, status, limit, time.Now().UTC())
}

func (s *Store) listAdminPurchasesPageAt(ctx context.Context, cursor, search, status string, limit int, now time.Time) ([]model.Purchase, *string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := purchaseSelect + ` WHERE 1=1`
	args := make([]any, 0, 8)
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		query += fmt.Sprintf(" AND (purchases.id ILIKE $%d ESCAPE '\\' \n\t\t\tOR purchases.user_id ILIKE $%d ESCAPE '\\' \n\t\t\tOR purchases.combo_id ILIKE $%d ESCAPE '\\' \n\t\t\tOR combos.name ILIKE $%d ESCAPE '\\' )", len(args)+1, len(args)+2, len(args)+3, len(args)+4)

		args = append(args, pattern, pattern, pattern, pattern)
	}
	if status != "" {
		// Raw lifecycle status remains available to settlement and advanced edits.
		query += fmt.Sprintf(" AND (CASE\n\t\t\tWHEN purchases.status IN ('active','activating','queued') AND "+purchaseTimestampOrderSQL("purchases.valid_until")+"<=$%d THEN 'expired'\n\t\t\tWHEN purchases.status IN ('active','activating') AND "+purchaseTimestampOrderSQL("purchases.valid_from")+">$%d THEN 'queued'\n\t\t\tELSE purchases.status END)=$%d", len(args)+1, len(args)+2, len(args)+3)

		args = append(args, purchaseTimestampOrder(now), purchaseTimestampOrder(now), status)
	}
	filter := pageFilterFingerprint(search, status)
	orderedCreated := purchaseTimestampOrderSQL("purchases.created_at")
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return nil, nil, err
		}
		orderedCursor := purchaseTimestampOrderSQL(fmt.Sprintf("$%d", len(args)+1))
		query += fmt.Sprintf(" AND (%s,purchases.id)<(%s,$%d)", orderedCreated, orderedCursor, len(args)+2)
		args = append(args, decoded.Timestamp, decoded.ID)
	}
	query += fmt.Sprintf(" ORDER BY %s DESC,purchases.id DESC LIMIT $%d", orderedCreated, len(args)+1)

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
	if err := hydratePurchaseSquads(ctx, s.db, items); err != nil {
		return nil, nil, err
	}
	if len(items) <= limit {
		return items, nil, nil
	}
	items = items[:limit]
	last := items[len(items)-1]
	next, err := encodeTimestampCursor(last.CreatedAt, last.ID, filter)
	return items, &next, err
}

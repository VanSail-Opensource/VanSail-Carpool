package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ListAdminPaymentOrdersPage returns a filtered stable page of payment attempts.
func (s *Store) ListAdminPaymentOrdersPage(ctx context.Context, cursor, search, status string, limit int) ([]model.PaymentOrder, *string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := paymentSelect + ` WHERE 1=1`
	args := make([]any, 0, 10)
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		query += fmt.Sprintf(" AND (id ILIKE $%d ESCAPE '\\'  OR user_id ILIKE $%d ESCAPE '\\' \n\t\t\tOR provider ILIKE $%d ESCAPE '\\'  OR COALESCE(method_id,'') ILIKE $%d ESCAPE '\\' \n\t\t\tOR COALESCE(provider_trade_id,'') ILIKE $%d ESCAPE '\\' \n\t\t\tOR COALESCE(provider_charge_id,'') ILIKE $%d ESCAPE '\\' )", len(args)+1, len(args)+2, len(args)+3, len(args)+4, len(args)+5, len(args)+6)

		args = append(args, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if status != "" {
		query += fmt.Sprintf(" AND status=$%d", len(args)+1)

		args = append(args, status)
	}
	filter := pageFilterFingerprint(search, status)
	orderedCreated := purchaseTimestampOrderSQL("created_at")
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return nil, nil, err
		}
		orderedCursor := purchaseTimestampOrderSQL(fmt.Sprintf("$%d", len(args)+1))
		query += fmt.Sprintf(" AND (%s,id)<(%s,$%d)", orderedCreated, orderedCursor, len(args)+2)
		args = append(args, decoded.Timestamp, decoded.ID)
	}
	query += fmt.Sprintf(" ORDER BY %s DESC,id DESC LIMIT $%d", orderedCreated, len(args)+1)

	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list admin payment page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]model.PaymentOrder, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanPaymentOrder(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
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

// ListAdminRefundsPage returns a filtered stable page of immutable refunds.
func (s *Store) ListAdminRefundsPage(ctx context.Context, cursor, search, status string, limit int) ([]model.Refund, *string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := `SELECT id,payment_order_id,actor_user_id,txb_minor,reason,status,created_at FROM refunds WHERE 1=1`
	args := make([]any, 0, 8)
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		query += fmt.Sprintf(" AND (id ILIKE $%d ESCAPE '\\'  OR payment_order_id ILIKE $%d ESCAPE '\\' \n\t\t\tOR COALESCE(actor_user_id,'') ILIKE $%d ESCAPE '\\'  OR reason ILIKE $%d ESCAPE '\\' )", len(args)+1, len(args)+2, len(args)+3, len(args)+4)

		args = append(args, pattern, pattern, pattern, pattern)
	}
	if status != "" {
		query += fmt.Sprintf(" AND status=$%d", len(args)+1)

		args = append(args, status)
	}
	filter := pageFilterFingerprint(search, status)
	orderedCreated := purchaseTimestampOrderSQL("created_at")
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return nil, nil, err
		}
		orderedCursor := purchaseTimestampOrderSQL(fmt.Sprintf("$%d", len(args)+1))
		query += fmt.Sprintf(" AND (%s,id)<(%s,$%d)", orderedCreated, orderedCursor, len(args)+2)
		args = append(args, decoded.Timestamp, decoded.ID)
	}
	query += fmt.Sprintf(" ORDER BY %s DESC,id DESC LIMIT $%d", orderedCreated, len(args)+1)

	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list admin refund page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]model.Refund, 0, limit+1)
	for rows.Next() {
		var item model.Refund
		var actor sql.NullString
		var txbMinor int64
		var created string
		if err := rows.Scan(&item.ID, &item.PaymentOrderID, &actor, &txbMinor, &item.Reason, &item.Status, &created); err != nil {
			return nil, nil, err
		}
		item.ActorUserID = nullableString(actor)
		item.TXB = model.TXBMoney(txbMinor)
		item.CreatedAt, err = parseStamp(created)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
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

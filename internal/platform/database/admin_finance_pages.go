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
		query += ` AND (id LIKE ? ESCAPE '\' COLLATE NOCASE OR user_id LIKE ? ESCAPE '\' COLLATE NOCASE
			OR provider LIKE ? ESCAPE '\' COLLATE NOCASE OR COALESCE(method_id,'') LIKE ? ESCAPE '\' COLLATE NOCASE
			OR COALESCE(provider_trade_id,'') LIKE ? ESCAPE '\' COLLATE NOCASE
			OR COALESCE(provider_charge_id,'') LIKE ? ESCAPE '\' COLLATE NOCASE)`
		args = append(args, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	filter := pageFilterFingerprint(search, status)
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return nil, nil, err
		}
		query += ` AND (created_at<? OR (created_at=? AND id<?))`
		args = append(args, decoded.Timestamp, decoded.Timestamp, decoded.ID)
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
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
		query += ` AND (id LIKE ? ESCAPE '\' COLLATE NOCASE OR payment_order_id LIKE ? ESCAPE '\' COLLATE NOCASE
			OR COALESCE(actor_user_id,'') LIKE ? ESCAPE '\' COLLATE NOCASE OR reason LIKE ? ESCAPE '\' COLLATE NOCASE)`
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	filter := pageFilterFingerprint(search, status)
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return nil, nil, err
		}
		query += ` AND (created_at<? OR (created_at=? AND id<?))`
		args = append(args, decoded.Timestamp, decoded.Timestamp, decoded.ID)
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
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

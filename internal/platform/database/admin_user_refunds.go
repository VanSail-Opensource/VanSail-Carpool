package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ListRefundsForUser returns immutable payment reversals for one aggregate profile.
func (s *Store) ListRefundsForUser(ctx context.Context, userID string, limit int) ([]model.Refund, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT refunds.id,refunds.payment_order_id,refunds.actor_user_id,
		refunds.txb_minor,refunds.reason,refunds.status,refunds.created_at FROM refunds
		JOIN payment_orders ON payment_orders.id=refunds.payment_order_id
		WHERE payment_orders.user_id=? ORDER BY refunds.created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]model.Refund, 0)
	for rows.Next() {
		var item model.Refund
		var actor sql.NullString
		var amount int64
		var created string
		if err := rows.Scan(&item.ID, &item.PaymentOrderID, &actor, &amount, &item.Reason, &item.Status, &created); err != nil {
			return nil, err
		}
		item.ActorUserID = nullableString(actor)
		item.TXB = model.TXBMoney(amount)
		item.CreatedAt, err = parseStamp(created)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

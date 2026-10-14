package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) paymentStatusShares(ctx context.Context) ([]model.NamedShare, error) {
	rows, err := s.db.QueryContext(ctx, `WITH payment_facts(provider,terminal,order_count) AS (
		SELECT provider,CASE
			WHEN status='refunded' THEN 'refunded'
			WHEN status='failed' THEN 'failed'
			WHEN status='paid' THEN 'paid'
			WHEN cancelled_at IS NOT NULL THEN 'cancelled'
			WHEN status='expired' THEN 'expired'
			ELSE '' END,1
		FROM payment_orders WHERE provider IN ('bepusdt','ezpay','builtin_usdt')
		UNION ALL
		SELECT provider,status,order_count FROM payment_status_rollups
		WHERE provider IN ('bepusdt','ezpay','builtin_usdt'))
	SELECT provider,terminal,SUM(order_count) FROM payment_facts
	WHERE terminal IN ('paid','expired','cancelled')
	GROUP BY provider,terminal ORDER BY provider,terminal`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]model.NamedShare, 0, 6)
	for rows.Next() {
		var provider, status string
		var count float64
		if err := rows.Scan(&provider, &status, &count); err != nil {
			return nil, err
		}
		id := provider + ":" + status
		result = append(result, model.NamedShare{ID: id, Label: id, Value: count})
	}
	return result, rows.Err()
}

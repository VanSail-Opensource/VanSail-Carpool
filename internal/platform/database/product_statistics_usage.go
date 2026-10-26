package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ActiveMemberUsageForStatistics returns one current combo and provider identity
// per non-admin member without a per-purchase owner query.
func (s *Store) ActiveMemberUsageForStatistics(ctx context.Context, now time.Time) ([]model.StatisticsUsageMember, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT purchases.id,purchases.user_id,purchases.charged_txb_minor,
		purchases.valid_from,purchases.valid_until,purchases.auto_renew_enabled,
		'xn:'||CAST(native.proxy_user_id AS TEXT)
		FROM purchases JOIN combos ON combos.id=purchases.combo_id JOIN users member ON member.id=purchases.user_id
		JOIN xnode_identities native ON native.user_id=member.id
		WHERE member.role='user'
		AND purchases.status IN ('active','activating') AND purchases.valid_from<=? AND purchases.valid_until>?
		ORDER BY purchases.user_id,purchases.valid_from DESC,purchases.created_at DESC`, stamp(now), stamp(now))
	if err != nil {
		return nil, fmt.Errorf("list active member purchases for statistics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	seen := make(map[string]struct{})
	result := make([]model.StatisticsUsageMember, 0)
	for rows.Next() {
		var item model.StatisticsUsageMember
		var validFrom, validUntil string
		var autoRenew int
		if err := rows.Scan(&item.Purchase.ID, &item.Purchase.UserID, &item.Purchase.PriceTXBMinor,
			&validFrom, &validUntil, &autoRenew, &item.RemoteUserID); err != nil {
			return nil, fmt.Errorf("scan active member usage: %w", err)
		}
		if _, exists := seen[item.Purchase.UserID]; exists {
			continue
		}
		item.Purchase.ValidFrom, err = parseStamp(validFrom)
		if err != nil {
			return nil, err
		}
		item.Purchase.ValidUntil, err = parseStamp(validUntil)
		if err != nil {
			return nil, err
		}
		item.Purchase.AutoRenewEnabled = autoRenew == 1
		seen[item.Purchase.UserID] = struct{}{}
		result = append(result, item)
	}
	return result, rows.Err()
}

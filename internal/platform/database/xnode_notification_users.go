package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// XNodeNotificationUsers reads one bounded native-owner page. The extra allowed
// row lets callers detect a full 1000-member page without loading the inventory.
// Each result carries the exact lifecycle and period for the enqueue fence.
func (s *Store) XNodeNotificationUsers(ctx context.Context, after int64, limit int) ([]notifications.TrafficUser, error) {
	if after < 0 || limit < 1 || limit > 1001 {
		return nil, xnode.ErrInvalid
	}
	rows, err := s.db.QueryContext(ctx, `WITH moment AS (
 SELECT to_char(CURRENT_TIMESTAMP AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US')||'000' AS at
 ) SELECT i.proxy_user_id,u.id,u.account_generation,p.id,
 p.used_bytes,p.limit_bytes,p.strategy,p.last_reset_at
 FROM xnode_identities i
 JOIN users u ON u.id=i.user_id
 JOIN xnode_periods p ON p.id=i.current_period_id AND p.proxy_user_id=i.proxy_user_id
 JOIN purchases b ON b.id=p.purchase_id AND b.user_id=u.id
 CROSS JOIN moment
 WHERE i.proxy_user_id>$1 AND i.credentials_cipher<>'' AND i.access_disabled=0
 AND u.account_status='active' AND u.onboarding_state='complete'
 AND p.status='open' AND b.status='active'
 AND `+purchaseTimestampOrderSQL("b.valid_from")+` <= moment.at
 AND `+purchaseTimestampOrderSQL("b.valid_until")+` > moment.at
 ORDER BY i.proxy_user_id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]notifications.TrafficUser, 0, limit)
	for rows.Next() {
		var item notifications.TrafficUser
		var reset sql.NullString
		if err = rows.Scan(&item.ID, &item.OwnerID, &item.AccountGeneration, &item.PeriodID,
			&item.UsedBytes, &item.LimitBytes, &item.ResetStrategy, &reset); err != nil {
			return nil, err
		}
		if reset.Valid {
			at, e := parseStamp(reset.String)
			if e != nil {
				return nil, e
			}
			item.LastTrafficResetAt = &at
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

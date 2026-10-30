package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type groupUsageReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// A dormant unfunded PAYG tuple is history, not membership. Its accounting rows
// survive deletion. Closing/review periods are an explicit unfinished barrier.
const groupUsageQuery = `WITH selected_purchases AS (
 SELECT ` + purchaseSquadColumns + ` FROM purchases
), ` + purchaseSquadMembershipCTE + `, members AS (
 SELECT o.group_id,o.user_id FROM owned_squads o JOIN purchases p ON p.id=o.purchase_id
 WHERE p.status IN ('active','activating','queued')
 UNION
 SELECT a.group_id,a.user_id FROM xnode_payg_accounts a
 JOIN xnode_periods p ON p.id=a.period_id JOIN users u ON u.id=a.user_id
 JOIN balances b ON b.user_id=a.user_id
 LEFT JOIN user_preferences prefs ON prefs.user_id=a.user_id
 LEFT JOIN squad_product_overrides s ON s.group_id=a.group_id
 LEFT JOIN pricing_product_settings pricing ON pricing.product_type='payg' AND pricing.product_id=a.group_id
 WHERE (a.credit_bytes>0 AND julianday(a.credit_started_at)>julianday(?)-3)
 OR (p.status='open' AND u.account_status='active' AND u.onboarding_state='complete'
 AND COALESCE(prefs.payg_enabled,1)=1 AND s.billing_mode='payg' AND s.payg_state<>'unavailable'
 AND NOT EXISTS(SELECT 1 FROM user_disabled_payg_squads d WHERE d.user_id=a.user_id AND d.group_id=a.group_id)
 AND b.txb_minor>=CASE WHEN pricing.dynamic_enabled=1 AND pricing.purchase_pricing_mode='DYNAMIC' THEN a.unit_price_minor ELSE COALESCE(NULLIF(s.payg_price_minor,0),a.unit_price_minor) END)
), unfinished AS (
 SELECT o.group_id FROM owned_squads o JOIN xnode_periods p ON p.purchase_id=o.purchase_id WHERE p.status<>'closed'
 UNION SELECT a.group_id FROM xnode_payg_accounts a JOIN xnode_periods p ON p.id=a.period_id WHERE p.status IN ('closing','review')
)
SELECT g.id,COUNT(DISTINCT m.user_id),EXISTS(SELECT 1 FROM unfinished f WHERE f.group_id=g.id)
FROM xnode_groups g LEFT JOIN members m ON m.group_id=g.id GROUP BY g.id`

func readGroupUsage(ctx context.Context, q groupUsageReader, now time.Time) (map[string]xnode.GroupUsage, error) {
	rows, err := q.QueryContext(ctx, groupUsageQuery, stamp(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	usage := map[string]xnode.GroupUsage{}
	for rows.Next() {
		var id string
		var u xnode.GroupUsage
		if err = rows.Scan(&id, &u.Members, &u.PendingAccounting); err != nil {
			return nil, err
		}
		usage[id] = u
	}
	return usage, rows.Err()
}

func (s *Store) XNodeGroupUsage(ctx context.Context, now time.Time) (map[string]xnode.GroupUsage, error) {
	return readGroupUsage(ctx, s.db, now)
}

package database

import (
	"context"
	"time"
)

// Only a routing index of grant identifiers is cached. Each applicable preview
// and every financial debit independently checks current policy, expiry and spend.
func (s *Store) pricingGrantScopes(ctx context.Context, now time.Time) (map[string]bool, error) {
	ids, err := pricingCached(ctx, s, "personal-grant-scopes", 30*time.Second, func() ([]string, error) {
		rows, e := s.db.QueryContext(ctx, `SELECT scope_id FROM pricing_admin_overrides WHERE command='PERSONAL_DISCOUNT' AND status='ACTIVE' AND starts_at<=? AND expires_at>? ORDER BY scope_id LIMIT 16384`, pricingStamp(now), pricingStamp(now))
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		ids := []string{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return nil, e
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

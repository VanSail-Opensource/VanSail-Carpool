package database

import (
	"context"
	"strings"
)

func (s *Store) PricingScopeVersion(ctx context.Context, kind, id string) (int64, error) {
	if kind == "product" {
		parts := strings.SplitN(id, ":", 2)
		if len(parts) != 2 {
			return 0, ErrConflict
		}
		p, err := s.PricingPolicy(ctx, parts[0], parts[1])
		return p.Version, err
	}
	var version int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(expected_version),0) FROM pricing_admin_overrides WHERE scope_type=? AND scope_id=?`, kind, id).Scan(&version)
	return version, err
}

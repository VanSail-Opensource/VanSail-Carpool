package database

import "context"

func (s *Store) XNodeWhitelistQuota(ctx context.Context, purchase string) (int, error) {
	var quota int
	err := s.db.QueryRowContext(ctx, `SELECT c.whitelist_ip_quota FROM combos c JOIN purchases p ON p.combo_id=c.id WHERE p.id=?`, purchase).Scan(&quota)
	return quota, err
}

package database

import "context"

// LegacyTrafficResetPaid proves an already committed financial obligation. No
// quote, preference or generic provider operation can authorize another reset.
func (s *Store) LegacyTrafficResetPaid(ctx context.Context, operation, owner string) (bool, error) {
	var paid bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ledger_entries l JOIN provider_operations o ON o.id=l.reference_id WHERE o.id=$1 AND o.owner_user_id=$2 AND o.kind='purchase_traffic_reset' AND l.user_id=$2 AND l.kind='traffic_reset_debit' AND l.delta_txb_minor<0)`, operation, owner).Scan(&paid)
	return paid, err
}

package database

import "context"

// PricingIncidentNotice returns sanitized incident facts for the queued alert.
func (s *Store) PricingIncidentNotice(ctx context.Context, id string) (string, string, string, error) {
	var scope, state, reason string
	err := s.db.QueryRowContext(ctx, `SELECT scope_id,state,reason FROM pricing_operational_incidents WHERE id=?`, id).Scan(&scope, &state, &reason)
	return scope, state, reason, err
}

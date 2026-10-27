package database

import (
	"context"
	"database/sql"
)

// PricingIncidentNotice returns sanitized incident facts for the queued alert.
func (s *Store) PricingIncidentNotice(ctx context.Context, id string) (string, string, string, bool, error) {
	var scope, state, reason string
	var ended sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT scope_id,state,reason,ended_at FROM pricing_operational_incidents WHERE id=?`, id).Scan(&scope, &state, &reason, &ended)
	return scope, state, reason, !ended.Valid, err
}

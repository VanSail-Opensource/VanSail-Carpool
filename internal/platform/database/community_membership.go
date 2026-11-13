package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// HasActiveCombo retains the public compatibility name for member eligibility:
// a current active combo or any actual PAYG purchase grants community access.
// An account, positive wallet or free unbilled traffic alone does not qualify.
func (s *Store) HasActiveCombo(ctx context.Context, userID string, now time.Time) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, ErrConflict
	}
	eligible, err := memberEligibility(ctx, s.db, userID, now)
	if err != nil {
		return false, fmt.Errorf("check member community access: %w", err)
	}
	return eligible, nil
}

// Preserve exact nanosecond boundaries when retained RFC3339Nano strings use
// different fractional widths. A paid PAYG row is an independent qualification.
func memberEligibility(ctx context.Context, reader purchaseSquadQueryer, userID string, now time.Time) (bool, error) {
	lower, upper := purchaseWindowCandidateBounds(now)
	rows, err := reader.QueryContext(ctx, `SELECT valid_from,valid_until FROM purchases
		WHERE user_id=$1 AND status='active' AND valid_from<$2 AND valid_until>$3
		UNION ALL SELECT NULL::text,NULL::text WHERE EXISTS(
			SELECT 1 FROM xnode_payg_accounts WHERE user_id=$1 AND purchased_bytes>0)`, userID, upper, lower)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var from, until sql.NullString
		if err := rows.Scan(&from, &until); err != nil {
			return false, err
		}
		if !from.Valid && !until.Valid {
			return true, nil
		}
		start, err := parseStamp(from.String)
		if err != nil {
			return false, err
		}
		end, err := parseStamp(until.String)
		if err != nil {
			return false, err
		}
		if !now.Before(start) && now.Before(end) {
			return true, nil
		}
	}
	return false, rows.Err()
}

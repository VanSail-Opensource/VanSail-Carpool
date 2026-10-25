package database

import (
	"context"
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
	var eligible bool
	if err := s.db.QueryRowContext(ctx, memberEligibilitySQL, userID, stamp(now), stamp(now), userID).Scan(&eligible); err != nil {
		return false, fmt.Errorf("check member community access: %w", err)
	}
	return eligible, nil
}

const memberEligibilitySQL = `SELECT EXISTS(SELECT 1 FROM purchases
	WHERE user_id=? AND status='active' AND valid_from<=? AND valid_until>?)
	OR EXISTS(SELECT 1 FROM xnode_payg_accounts WHERE user_id=? AND purchased_bytes>0)`

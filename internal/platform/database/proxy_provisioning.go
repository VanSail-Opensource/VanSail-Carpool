package database

import (
	"context"
	"fmt"
	"time"
)

// HasProvisionablePurchase limits remote creation to paid, unexpired access.
func (s *Store) HasProvisionablePurchase(ctx context.Context, userID string, now time.Time) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT (EXISTS(SELECT 1 FROM purchases purchase JOIN users owner ON owner.id=purchase.user_id WHERE purchase.user_id=$1
		AND owner.account_status='active' AND purchase.status IN ('active','activating','queued') AND `+purchaseTimestampOrderSQL("purchase.valid_until")+`>$2))::integer`, userID, purchaseTimestampOrder(now)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check provisionable purchase: %w", err)
	}
	return exists == 1, nil
}

// LinkProvisionedProxyUser changes the provider ID only if local identity is stable.
func (s *Store) LinkProvisionedProxyUser(ctx context.Context, userID, username, previousID, remoteID string, now time.Time) error {
	if userID == "" || username == "" || remoteID == "" {
		return ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE users SET proxy_user_id=$1,recovery_reason='',updated_at=$2
		WHERE id=$3 AND CAST(telegram_id AS TEXT)=$4 AND onboarding_state='complete' AND account_status='active' AND COALESCE(proxy_user_id,'')=$5
		AND EXISTS(SELECT 1 FROM purchases WHERE user_id=$6 AND status IN ('active','activating','queued') AND `+purchaseTimestampOrderSQL("valid_until")+`>$7)`,
		remoteID, stamp(now), userID, username, previousID, userID, purchaseTimestampOrder(now))
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrConflict
		}
		return fmt.Errorf("link provisioned Proxy user: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected != 1 {
		return ErrConflict
	}
	return nil
}

// ResolveProvisioningConflict leaves paid access and accepted agreements intact.
// A Telegram identity mismatch requires provider review, never a new username.
func (s *Store) ResolveProvisioningConflict(ctx context.Context, userID, username string, now time.Time) error {
	return ErrConflict
}

package database

import (
	"context"
	"fmt"
	"time"
)

// HasProvisionablePurchase limits remote creation to paid, unexpired access.
func (s *Store) HasProvisionablePurchase(ctx context.Context, userID string, now time.Time) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=?
		AND status IN ('active','activating','queued') AND valid_until>?)`, userID, stamp(now)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check provisionable purchase: %w", err)
	}
	return exists == 1, nil
}

// LinkProvisionedRemnaUser changes the provider ID only if local identity is stable.
func (s *Store) LinkProvisionedRemnaUser(ctx context.Context, userID, username, previousID, remoteID string, now time.Time) error {
	if userID == "" || username == "" || remoteID == "" {
		return ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE users SET remna_user_id=?,remna_subscription_url=NULL,recovery_reason='',updated_at=?
		WHERE id=? AND CAST(telegram_id AS TEXT)=? AND onboarding_state='complete' AND COALESCE(remna_user_id,'')=?
		AND EXISTS(SELECT 1 FROM purchases WHERE user_id=? AND status IN ('active','activating','queued') AND valid_until>?)`,
		remoteID, stamp(now), userID, username, previousID, userID, stamp(now))
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrConflict
		}
		return fmt.Errorf("link provisioned Remnawave user: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected != 1 {
		return ErrConflict
	}
	return nil
}

type conflictedPurchase struct {
	id, name string
	charged  int64
}

// ResolveProvisioningConflict leaves paid access and accepted agreements intact.
// A Telegram identity mismatch requires provider review, never a new username.
func (s *Store) ResolveProvisioningConflict(ctx context.Context, userID, username string, now time.Time) error {
	return ErrConflict
}

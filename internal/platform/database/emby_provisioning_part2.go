package database

import (
	"context"
	"fmt"
	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"time"
)

func (s *Store) retryAttempt_FailAndRefundEmbySetup(ctx context.Context, accountID, reason string, now time.Time) (domain.Account, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Account{}, err
	}
	defer func() { _ = tx.Rollback() }()
	account, err := failAndRefundEmbySetupTx(ctx, tx, accountID, reason, now, nil)
	if err != nil {
		return domain.Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Account{}, fmt.Errorf("commit Emby setup refund: %w", err)
	}
	return account, nil
}

// UpdateEmbyPreferences persists only preferences that were accepted upstream.

func (s *Store) retryAttempt_UpdateEmbyPreferences(ctx context.Context, accountID string, preferences domain.Preferences, now time.Time) (domain.Account, error) {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Account{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET max_parental_rating=$1,updated_at=$2 WHERE id=$3 AND status='active'`,
		nullableInt32(preferences.MaxParentalRating), stamp(now), accountID)
	if err != nil {
		return domain.Account{}, fmt.Errorf("update Emby preferences: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.Account{}, domain.ErrNotFound
	}
	if err := replaceEmbyFoldersTx(ctx, tx, accountID, preferences.DisabledLibraryIDs); err != nil {
		return domain.Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Account{}, err
	}
	record, err := s.EmbyProvisioningByID(ctx, accountID)
	return record.Account, err
}

// TouchEmbyAccount records a successful password operation without storing password data.

func (s *Store) TouchEmbyAccount(ctx context.Context, accountID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE emby_accounts SET updated_at=$1 WHERE id=$2 AND status='active'`, stamp(now), accountID)
	if err != nil {
		return fmt.Errorf("touch Emby account: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

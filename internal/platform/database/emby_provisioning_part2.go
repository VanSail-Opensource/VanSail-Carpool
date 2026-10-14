package database

import (
	"context"
	"fmt"
	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"time"
)

func (s *Store) FailAndRefundEmbySetup(ctx context.Context, accountID, reason string, now time.Time) (domain.Account, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Account{}, err
	}
	defer func() { _ = tx.Rollback() }()
	record, err := scanEmbyProvisioning(tx.QueryRowContext(ctx, embyAccountSelect+` WHERE id=?`, accountID))
	if err != nil {
		return domain.Account{}, err
	}
	if record.Status == domain.StatusActive {
		return domain.Account{}, ErrConflict
	}
	if record.RefundedAt != nil {
		err = hydrateEmbyPreferences(ctx, tx, &record)
		return record.Account, err
	}
	balance, err := adjustBalanceTx(ctx, tx, record.UserID, record.SetupPriceTXBMinor, now)
	if err != nil {
		return domain.Account{}, fmt.Errorf("refund Emby setup balance: %w", err)
	}
	referenceID := fmt.Sprintf("%s:%d", record.ID, record.SetupAttempt)
	if _, err := insertLedgerTx(ctx, tx, record.UserID, record.SetupPriceTXBMinor, balance, "emby_setup_refund", referenceID, "Emby setup refund", now); err != nil {
		return domain.Account{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET status='failed',password_ciphertext='',password_context='',pending_preferences_json='{}',
		last_error=?,refunded_at=?,updated_at=? WHERE id=?`, truncateEmbyError(reason), stamp(now), stamp(now), record.ID); err != nil {
		return domain.Account{}, fmt.Errorf("record failed Emby setup: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Account{}, fmt.Errorf("commit Emby setup refund: %w", err)
	}
	return s.EmbyAccountForUser(ctx, record.UserID)
}

// UpdateEmbyPreferences persists only preferences that were accepted upstream.

func (s *Store) UpdateEmbyPreferences(ctx context.Context, accountID string, preferences domain.Preferences, now time.Time) (domain.Account, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Account{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET max_parental_rating=?,updated_at=? WHERE id=? AND status='active'`,
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
	result, err := s.db.ExecContext(ctx, `UPDATE emby_accounts SET updated_at=? WHERE id=? AND status='active'`, stamp(now), accountID)
	if err != nil {
		return fmt.Errorf("touch Emby account: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

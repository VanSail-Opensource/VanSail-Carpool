package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

// FailAndRefundEmbySetupIfUnchanged binds the service's absence/failure proof to
// the current setup attempt. The raw helper remains available to historical
// fixtures; production provisioning supplies its exact reviewed snapshot.
func (s *Store) retryAttempt_FailAndRefundEmbySetupIfUnchanged(ctx context.Context, expected domain.ProvisioningRecord, reason string, now time.Time) (domain.Account, error) {
	if expected.ID == "" || expected.UserID == "" || expected.SetupAttempt <= 0 || expected.UpdatedAt.IsZero() {
		return domain.Account{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Account{}, err
	}
	defer func() { _ = tx.Rollback() }()
	account, err := failAndRefundEmbySetupTx(ctx, tx, expected.ID, reason, now, &expected)
	if err != nil {
		return domain.Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Account{}, fmt.Errorf("commit Emby setup refund: %w", err)
	}
	return account, nil
}

func failAndRefundEmbySetupTx(ctx context.Context, tx *sql.Tx, accountID, reason string, now time.Time, expected *domain.ProvisioningRecord) (domain.Account, error) {
	record, err := scanEmbyProvisioning(tx.QueryRowContext(ctx, embyAccountSelect+` WHERE id=$1`, accountID))
	if err != nil {
		return domain.Account{}, err
	}
	if err := lockPurchaseOwnerTx(ctx, tx, record.UserID); err != nil {
		return domain.Account{}, err
	}
	// Match setup and purge: wallet before the mutable Emby account.
	if _, err := balanceTx(ctx, tx, record.UserID); err != nil {
		return domain.Account{}, err
	}
	record, err = scanEmbyProvisioning(tx.QueryRowContext(ctx, embyAccountSelect+` WHERE id=$1 FOR UPDATE`, accountID))
	if err != nil {
		return domain.Account{}, err
	}
	if expected != nil && (record.UserID != expected.UserID || record.SetupAttempt != expected.SetupAttempt ||
		record.SetupPriceTXBMinor != expected.SetupPriceTXBMinor) {
		return domain.Account{}, ErrConflict
	}
	if record.Status == domain.StatusActive {
		return domain.Account{}, ErrConflict
	}
	if record.RefundedAt != nil {
		err = hydrateEmbyPreferences(ctx, tx, &record)
		return record.Account, err
	}
	if expected != nil && !sameEmbyProvisioningSnapshot(record, *expected) {
		return domain.Account{}, ErrConflict
	}
	balance, err := adjustBalanceTx(ctx, tx, record.UserID, record.SetupPriceTXBMinor, now)
	if err != nil {
		return domain.Account{}, fmt.Errorf("refund Emby setup balance: %w", err)
	}
	referenceID := fmt.Sprintf("%s:%d", record.ID, record.SetupAttempt)
	if _, err := insertLedgerTx(ctx, tx, record.UserID, record.SetupPriceTXBMinor, balance,
		"emby_setup_refund", referenceID, "Emby setup refund", now); err != nil {
		return domain.Account{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET status='failed',password_ciphertext='',password_context='',pending_preferences_json='{}',
		last_error=$1,refunded_at=$2,updated_at=$3 WHERE id=$4`, truncateEmbyError(reason), stamp(now), stamp(now), record.ID); err != nil {
		return domain.Account{}, fmt.Errorf("record failed Emby setup: %w", err)
	}
	record, err = scanEmbyProvisioning(tx.QueryRowContext(ctx, embyAccountSelect+` WHERE id=$1`, accountID))
	if err != nil {
		return domain.Account{}, err
	}
	if err := hydrateEmbyPreferences(ctx, tx, &record); err != nil {
		return domain.Account{}, err
	}
	return record.Account, nil
}

func sameEmbyProvisioningSnapshot(current, expected domain.ProvisioningRecord) bool {
	return current.ID == expected.ID && current.UserID == expected.UserID &&
		current.SetupAttempt == expected.SetupAttempt && current.SetupPriceTXBMinor == expected.SetupPriceTXBMinor &&
		current.Status == expected.Status && current.UpdatedAt.Equal(expected.UpdatedAt) &&
		current.BaseUsername == expected.BaseUsername && current.RemoteUserID == expected.RemoteUserID &&
		current.RemoteUsername == expected.RemoteUsername && current.CandidateUsername == expected.CandidateUsername &&
		current.CreateAttempted == expected.CreateAttempted && current.LastError == expected.LastError &&
		current.PasswordCiphertext == expected.PasswordCiphertext && current.PasswordContext == expected.PasswordContext &&
		current.PendingPreferencesJSON == expected.PendingPreferencesJSON
}

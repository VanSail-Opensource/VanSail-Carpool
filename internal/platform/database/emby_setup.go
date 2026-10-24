package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

// QueueEmbySetup atomically debits setup and queues the legacy provisioning lane.
func (s *Store) retryAttempt_QueueEmbySetup(ctx context.Context, input domain.QueueSetupInput, now time.Time) (domain.Account, bool, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Account{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	account, created, err := queueEmbySetupTx(ctx, tx, input, now.UTC(), true)
	if err != nil {
		return domain.Account{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Account{}, false, fmt.Errorf("commit Emby setup: %w", err)
	}
	return account, created, nil
}

func queueEmbySetupTx(ctx context.Context, tx *sql.Tx, input domain.QueueSetupInput, now time.Time,
	legacyOutbox bool) (domain.Account, bool, error) {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.UserID) == "" ||
		strings.TrimSpace(input.BaseUsername) == "" || input.SetupPriceTXBMinor < 0 ||
		input.PasswordCiphertext == "" || input.PasswordContext == "" {
		return domain.Account{}, false, domain.ErrInvalidSetup
	}
	var canonicalUsername string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT base_username FROM emby_accounts WHERE user_id=users.id),CAST(telegram_id AS TEXT)) FROM users WHERE id=?`,
		input.UserID).Scan(&canonicalUsername); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Account{}, false, ErrNotFound
		}
		return domain.Account{}, false, fmt.Errorf("validate Emby setup user: %w", err)
	}
	if canonicalUsername != input.BaseUsername {
		return domain.Account{}, false, ErrConflict
	}
	existing, loadErr := scanEmbyProvisioning(tx.QueryRowContext(ctx, embyAccountSelect+` WHERE user_id=?`, input.UserID))
	accountID, attempt, isRetry := input.ID, 1, false
	if loadErr == nil {
		if err := hydrateEmbyPreferences(ctx, tx, &existing); err != nil {
			return domain.Account{}, false, err
		}
		if existing.Status != domain.StatusFailed || existing.RefundedAt == nil {
			return existing.Account, false, nil
		}
		accountID, attempt, isRetry = existing.ID, existing.SetupAttempt+1, true
	} else if !errors.Is(loadErr, domain.ErrNotFound) {
		return domain.Account{}, false, loadErr
	}
	newBalance, err := debitBalanceTx(ctx, tx, input.UserID, input.SetupPriceTXBMinor, now)
	if err != nil {
		return domain.Account{}, false, err
	}
	pendingPreferences, err := json.Marshal(input.Preferences)
	if err != nil {
		return domain.Account{}, false, fmt.Errorf("encode pending Emby preferences: %w", err)
	}
	if err := writeEmbySetupTx(ctx, tx, input, accountID, attempt, isRetry, pendingPreferences, now); err != nil {
		return domain.Account{}, false, err
	}
	referenceID := fmt.Sprintf("%s:%d", accountID, attempt)
	if _, err := insertLedgerTx(ctx, tx, input.UserID, -input.SetupPriceTXBMinor, newBalance,
		"emby_setup_debit", referenceID, "Emby account setup", now); err != nil {
		return domain.Account{}, false, err
	}
	if legacyOutbox {
		payload, err := embyProvisionPayload(accountID, attempt)
		if err != nil {
			return domain.Account{}, false, err
		}
		if err := insertOutboxTx(ctx, tx, domain.ProvisionOutboxKind, string(payload), now, now); err != nil {
			return domain.Account{}, false, err
		}
	}
	record, err := scanEmbyProvisioning(tx.QueryRowContext(ctx, embyAccountSelect+` WHERE id=?`, accountID))
	if err != nil {
		return domain.Account{}, false, err
	}
	if err := hydrateEmbyPreferences(ctx, tx, &record); err != nil {
		return domain.Account{}, false, err
	}
	return record.Account, true, nil
}

func writeEmbySetupTx(ctx context.Context, tx *sql.Tx, input domain.QueueSetupInput, accountID string,
	attempt int, retry bool, preferences []byte, now time.Time) error {
	if retry {
		_, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET base_username=?,
			candidate_username=CASE WHEN remote_user_id IS NULL THEN NULL ELSE candidate_username END,
			remote_username=CASE WHEN remote_user_id IS NULL THEN NULL ELSE remote_username END,
			status='queued',setup_price_txb_minor=?,setup_attempt=?,password_ciphertext=?,password_context=?,
			pending_preferences_json=?,create_attempted=CASE WHEN remote_user_id IS NULL THEN 0 ELSE create_attempted END,
			last_error='',refunded_at=NULL,updated_at=? WHERE id=?`, input.BaseUsername, input.SetupPriceTXBMinor,
			attempt, input.PasswordCiphertext, input.PasswordContext, string(preferences), stamp(now), accountID)
		if err != nil {
			return fmt.Errorf("restart Emby setup: %w", err)
		}
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO emby_accounts(id,user_id,base_username,status,setup_price_txb_minor,
		setup_attempt,password_ciphertext,password_context,pending_preferences_json,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`, accountID, input.UserID, input.BaseUsername, domain.StatusQueued,
		input.SetupPriceTXBMinor, attempt, input.PasswordCiphertext, input.PasswordContext,
		string(preferences), stamp(now), stamp(now))
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("insert Emby setup: %w", err)
	}
	return nil
}

func embyProvisionPayload(accountID string, attempt int) ([]byte, error) {
	return json.Marshal(map[string]string{"accountId": accountID, "attempt": fmt.Sprintf("%d", attempt)})
}

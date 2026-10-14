package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// BeginEmbySetupOperation atomically stores the debit, account, and provider command.
func (s *Store) BeginEmbySetupOperation(ctx context.Context, command providerops.CreateInput,
	setup domain.QueueSetupInput, now time.Time) (providerops.Operation, bool, error) {
	if len(command.Items) != 1 || command.Items[0].TargetType != "emby_user" ||
		command.Items[0].TargetID != setup.UserID || command.OwnerUserID != setup.UserID {
		return providerops.Operation{}, false, ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	operation, replayed, err := createProviderOperationTx(ctx, tx, command, now.UTC())
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if replayed {
		return commitEmbyOperation(tx, operation, true)
	}
	if _, created, err := queueEmbySetupTx(ctx, tx, setup, now.UTC(), false); err != nil {
		return providerops.Operation{}, false, err
	} else if !created {
		return providerops.Operation{}, false, domain.ErrAccountExists
	}
	return commitEmbyOperation(tx, operation, false)
}

// BeginEmbyProvisionRetryOperation replaces inactive legacy work with one command.
func (s *Store) BeginEmbyProvisionRetryOperation(ctx context.Context, command providerops.CreateInput,
	accountID string, now time.Time) (providerops.Operation, bool, error) {
	if len(command.Items) != 1 || command.Items[0].TargetType != "emby_account" ||
		command.Items[0].TargetID != accountID {
		return providerops.Operation{}, false, ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	operation, replayed, err := createProviderOperationTx(ctx, tx, command, now.UTC())
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if replayed {
		return commitEmbyOperation(tx, operation, true)
	}
	record, err := scanEmbyProvisioning(tx.QueryRowContext(ctx, embyAccountSelect+` WHERE id=?`, accountID))
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if command.OwnerUserID != record.UserID || record.PasswordCiphertext == "" ||
		(record.Status != domain.StatusQueued && record.Status != domain.StatusPendingReview) {
		return providerops.Operation{}, false, ErrConflict
	}
	payload, err := embyProvisionPayload(accountID, record.SetupAttempt)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	var processing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND payload=? AND status='processing'`,
		domain.ProvisionOutboxKind, string(payload)).Scan(&processing); err != nil {
		return providerops.Operation{}, false, err
	}
	if processing != 0 {
		return providerops.Operation{}, false, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind=? AND payload=?`,
		domain.ProvisionOutboxKind, string(payload)); err != nil {
		return providerops.Operation{}, false, fmt.Errorf("remove legacy Emby retry work: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET status='queued',last_error='',updated_at=?
		WHERE id=? AND status IN ('queued','pending_review')`, stamp(now), accountID)
	if err != nil {
		return providerops.Operation{}, false, fmt.Errorf("queue durable Emby retry: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return providerops.Operation{}, false, rowsErr
		}
		return providerops.Operation{}, false, ErrConflict
	}
	return commitEmbyOperation(tx, operation, false)
}

func commitEmbyOperation(tx *sql.Tx, operation providerops.Operation,
	replayed bool) (providerops.Operation, bool, error) {
	if err := tx.Commit(); err != nil {
		return providerops.Operation{}, false, fmt.Errorf("commit Emby operation: %w", err)
	}
	return operation, replayed, nil
}

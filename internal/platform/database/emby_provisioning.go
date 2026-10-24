package database

import (
	"context"
	"database/sql"
	"fmt"
	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"time"
)

func (s *Store) retryAttempt_RetryEmbyProvisioning(ctx context.Context, accountID string, now time.Time) (domain.Account, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
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
		err = hydrateEmbyPreferences(ctx, tx, &record)
		return record.Account, err
	}
	if (record.Status != domain.StatusQueued && record.Status != domain.StatusPendingReview) || record.PasswordCiphertext == "" {
		return domain.Account{}, ErrConflict
	}
	if record.Status == domain.StatusPendingReview {
		if _, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET status='queued',last_error='',updated_at=?
			WHERE id=? AND status='pending_review'`, stamp(now), accountID); err != nil {
			return domain.Account{}, fmt.Errorf("resume Emby provisioning review: %w", err)
		}
	}
	payload, marshalErr := embyProvisionPayload(accountID, record.SetupAttempt)
	if marshalErr != nil {
		return domain.Account{}, marshalErr
	}
	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND payload=? AND status IN ('pending','processing')`,
		domain.ProvisionOutboxKind, string(payload)).Scan(&existing); err != nil {
		return domain.Account{}, fmt.Errorf("check Emby retry work: %w", err)
	}
	if existing == 0 {
		if err := insertOutboxTx(ctx, tx, domain.ProvisionOutboxKind, string(payload), now, now); err != nil {
			return domain.Account{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Account{}, fmt.Errorf("commit Emby retry: %w", err)
	}
	return s.EmbyAccountForUser(ctx, record.UserID)
}

// BeginEmbyProvisioning leases the account state to a kind-specific outbox handler.

func (s *Store) BeginEmbyProvisioning(ctx context.Context, accountID string, now time.Time) (domain.ProvisioningRecord, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := s.db.ExecContext(ctx, `UPDATE emby_accounts SET status='provisioning',updated_at=? WHERE id=? AND status='queued'`, stamp(now), accountID); err != nil {
		return domain.ProvisioningRecord{}, fmt.Errorf("begin Emby provisioning: %w", err)
	}
	return s.EmbyProvisioningByID(ctx, accountID)
}

// SetEmbyCandidate persists the exact name before any create attempt.

func (s *Store) SetEmbyCandidate(ctx context.Context, accountID, candidate string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE emby_accounts SET candidate_username=?,updated_at=?
		WHERE id=? AND status='provisioning' AND candidate_username IS NULL`, candidate, stamp(now), accountID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAccountExists
		}
		return fmt.Errorf("set Emby candidate: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var stored sql.NullString
		if loadErr := s.db.QueryRowContext(ctx, `SELECT candidate_username FROM emby_accounts WHERE id=?`, accountID).Scan(&stored); loadErr != nil {
			return mapEmbyNotFound(loadErr)
		}
		if !stored.Valid || stored.String != candidate {
			return domain.ErrAccountExists
		}
	}
	return nil
}

// MarkEmbyCreateAttempted records the ambiguity boundary before POST /Users/New.

func (s *Store) MarkEmbyCreateAttempted(ctx context.Context, accountID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE emby_accounts SET create_attempted=1,updated_at=?
		WHERE id=? AND status='provisioning' AND candidate_username IS NOT NULL`, stamp(now), accountID)
	if err != nil {
		return fmt.Errorf("mark Emby create attempt: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetEmbyRemoteIdentity stores the authoritative remote identity idempotently.

func (s *Store) SetEmbyRemoteIdentity(ctx context.Context, accountID, remoteID, remoteUsername string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE emby_accounts SET remote_user_id=?,remote_username=?,candidate_username=?,updated_at=?
		WHERE id=? AND status='provisioning' AND (remote_user_id IS NULL OR remote_user_id=?)`,
		remoteID, remoteUsername, remoteUsername, stamp(now), accountID, remoteID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAccountExists
		}
		return fmt.Errorf("set Emby remote identity: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.ErrAccountExists
	}
	return nil
}

// RequeueEmbyProvisioning preserves the secret and records a retry-safe error.

func (s *Store) RequeueEmbyProvisioning(ctx context.Context, accountID string, provisionErr error, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE emby_accounts SET status='queued',last_error=?,updated_at=?
		WHERE id=? AND status='provisioning'`, sanitizeError(provisionErr), stamp(now), accountID)
	if err != nil {
		return fmt.Errorf("requeue Emby provisioning: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkEmbyPendingReview stops automatic retries while retaining the sealed desired state.
func (s *Store) MarkEmbyPendingReview(ctx context.Context, accountID string, provisionErr error, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE emby_accounts SET status='pending_review',last_error=?,updated_at=?
		WHERE id=? AND status='provisioning'`, sanitizeError(provisionErr), stamp(now), accountID)
	if err != nil {
		return fmt.Errorf("mark Emby provisioning for review: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkEmbyProvisioned persists disabled folders only after the service has
// re-fetched and verified the remote full policy.

func (s *Store) retryAttempt_MarkEmbyProvisioned(ctx context.Context, accountID string, preferences domain.Preferences, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET status='active',password_ciphertext='',password_context='',
		max_parental_rating=?,pending_preferences_json='{}',last_error='',provisioned_at=COALESCE(provisioned_at,?),updated_at=?
		WHERE id=? AND status='provisioning' AND remote_user_id IS NOT NULL`, nullableInt32(preferences.MaxParentalRating), stamp(now), stamp(now), accountID)
	if err != nil {
		return fmt.Errorf("mark Emby provisioned: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrConflict
	}
	if err := replaceEmbyFoldersTx(ctx, tx, accountID, preferences.DisabledLibraryIDs); err != nil {
		return err
	}
	return tx.Commit()
}

// FailAndRefundEmbySetup atomically records a terminal failure, erases the
// temporary password, and credits the exact debited setup price once.

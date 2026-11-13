package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// RestoreEmbyRemoteIdentity replaces only the exact reviewed provisioning
// identity after the service proved the old ID absent and found its candidate.
// A completed purge deletes the account row; a later setup cannot reuse this ID.
func (s *Store) retryAttempt_RestoreEmbyRemoteIdentity(ctx context.Context, expected domain.ProvisioningRecord, remoteID, remoteName string, now time.Time) error {
	if expected.ID == "" || expected.UserID == "" || expected.SetupAttempt <= 0 || expected.UpdatedAt.IsZero() ||
		expected.Status != domain.StatusProvisioning || expected.RemoteUserID == "" || expected.CandidateUsername == "" ||
		strings.TrimSpace(remoteID) == "" || remoteID == expected.RemoteUserID || remoteName != expected.CandidateUsername {
		return ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockProviderOperationAccountsTx(ctx, tx, expected.UserID, ""); err != nil {
		return err
	}
	current, err := scanEmbyProvisioning(tx.QueryRowContext(ctx, embyAccountSelect+` WHERE id=$1 FOR UPDATE`, expected.ID))
	if errors.Is(err, domain.ErrNotFound) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if current.UserID != expected.UserID || current.SetupAttempt != expected.SetupAttempt {
		return ErrConflict
	}
	if current.RemoteUserID == remoteID && current.RemoteUsername == remoteName && current.CandidateUsername == remoteName &&
		(current.Status == domain.StatusProvisioning || current.Status == domain.StatusActive) {
		return nil
	}
	if !sameEmbyProvisioningSnapshot(current, expected) {
		return ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET remote_user_id=$1,remote_username=$2,updated_at=$3 WHERE id=$4`,
		remoteID, remoteName, stamp(now), expected.ID); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAccountExists
		}
		return fmt.Errorf("restore Emby identity: %w", err)
	}
	auditID, err := ids.New()
	if err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]any{
		"previousRemoteId": expected.RemoteUserID, "remoteId": remoteID, "setupAttempt": expected.SetupAttempt,
	})
	if err != nil {
		return err
	}
	if err := insertAuditTx(ctx, tx, auditID, nil, "emby.identity.restore", "emby_account", expected.ID, string(detail), now); err != nil {
		return err
	}
	return tx.Commit()
}

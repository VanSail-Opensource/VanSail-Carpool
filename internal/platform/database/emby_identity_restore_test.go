package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

func TestEmbyIdentityRestoreRequiresCurrentOwnedAttempt(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	expected := createEmbyRestoreTarget(t, store, 98_930, "restore-target", "old-remote")
	other := createEmbyRestoreTarget(t, store, 98_931, "other-target", "other-remote")
	now := expected.UpdatedAt
	stale := expected
	stale.UpdatedAt = now.Add(-time.Nanosecond)
	if err := store.RestoreEmbyRemoteIdentity(ctx, stale, "new-remote", expected.CandidateUsername, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale absence proof changed identity: %v", err)
	}
	if err := store.RestoreEmbyRemoteIdentity(ctx, expected, "new-remote", "different-name", now); !errors.Is(err, ErrConflict) {
		t.Fatalf("different candidate was adopted: %v", err)
	}
	if err := store.RestoreEmbyRemoteIdentity(ctx, expected, other.RemoteUserID, expected.CandidateUsername, now); !errors.Is(err, domain.ErrAccountExists) {
		t.Fatalf("another owner's identity was adopted: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, expected.UserID); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreEmbyRemoteIdentity(ctx, expected, "new-remote", expected.CandidateUsername, now); err == nil {
		t.Fatal("inactive owner acquired a replacement identity")
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status='active' WHERE id=$1`, expected.UserID); err != nil {
		t.Fatal(err)
	}
	current, err := store.EmbyProvisioningByID(ctx, expected.ID)
	if err != nil || current.RemoteUserID != expected.RemoteUserID {
		t.Fatal("rejected restores changed the original identity", err)
	}
	if _, err := store.DB().ExecContext(ctx, `ALTER TABLE audit_events ADD CONSTRAINT reject_emby_restore CHECK (action <> 'emby.identity.restore')`); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreEmbyRemoteIdentity(ctx, expected, "new-remote", expected.CandidateUsername, now); err == nil {
		t.Fatal("restore committed without its audit")
	}
	current, err = store.EmbyProvisioningByID(ctx, expected.ID)
	if err != nil || current.RemoteUserID != expected.RemoteUserID {
		t.Fatal("failed audit did not roll back identity", err)
	}
	if _, err := store.DB().ExecContext(ctx, `ALTER TABLE audit_events DROP CONSTRAINT reject_emby_restore`); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreEmbyRemoteIdentity(ctx, expected, "new-remote", expected.CandidateUsername, now); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreEmbyRemoteIdentity(ctx, expected, "new-remote", expected.CandidateUsername, now.Add(time.Second)); err != nil {
		t.Fatal("committed restoration did not replay", err)
	}
	if err := store.RestoreEmbyRemoteIdentity(ctx, expected, "later-remote", expected.CandidateUsername, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("old identity snapshot overwrote a restored target: %v", err)
	}
	current, err = store.EmbyProvisioningByID(ctx, expected.ID)
	if err != nil || current.RemoteUserID != "new-remote" || current.Status != domain.StatusProvisioning ||
		current.PasswordCiphertext != expected.PasswordCiphertext || !current.UpdatedAt.Equal(now) {
		t.Fatal("restoration changed unrelated setup evidence", err)
	}
	var count int
	var detail string
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),MIN(detail) FROM audit_events
		WHERE action='emby.identity.restore' AND target_type='emby_account' AND target_id=$1`, expected.ID).Scan(&count, &detail); err != nil || count != 1 {
		t.Fatalf("restore audit count=%d error=%v", count, err)
	}
	var evidence struct {
		Previous string `json:"previousRemoteId"`
		Current  string `json:"remoteId"`
	}
	if json.Unmarshal([]byte(detail), &evidence) != nil || evidence.Previous != expected.RemoteUserID || evidence.Current != "new-remote" {
		t.Fatal("restore did not retain the old and new identity evidence")
	}
	if _, err := store.FailAndRefundEmbySetup(ctx, expected.ID, "confirmed remote absence", now); err != nil {
		t.Fatal(err)
	}
	if _, created, err := store.QueueEmbySetup(ctx, domain.QueueSetupInput{
		ID: "unused", UserID: expected.UserID, BaseUsername: expected.BaseUsername,
		PasswordCiphertext: "next-setup", PasswordContext: expected.PasswordContext,
	}, now); err != nil || !created {
		t.Fatal("next setup fixture failed", err)
	}
	if err := store.RestoreEmbyRemoteIdentity(ctx, expected, "new-remote", expected.CandidateUsername, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("old-attempt replay crossed a later paid setup: %v", err)
	}
}

func createEmbyRestoreTarget(t *testing.T, store *Store, telegramID int64, id, remote string) domain.ProvisioningRecord {
	t.Helper()
	ctx, now := context.Background(), time.Now().UTC()
	owner := createTestUser(t, store, telegramID)
	account, _, err := store.QueueEmbySetup(ctx, domain.QueueSetupInput{
		ID: id, UserID: owner.ID, BaseUsername: owner.AccountName(),
		PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:" + owner.ID,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginEmbyProvisioning(ctx, account.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEmbyRemoteIdentity(ctx, account.ID, remote, owner.AccountName(), now); err != nil {
		t.Fatal(err)
	}
	record, err := store.EmbyProvisioningByID(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

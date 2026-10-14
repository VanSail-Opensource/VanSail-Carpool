package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestEmbySetupDebitRefundAndRetryAreAtomic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newEmbyTestStore(t)
	user := createEmbyTestUser(t, store, 31001)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET username='ada',onboarding_state='complete' WHERE id=?`, user.ID); err != nil {
		t.Fatalf("set test username: %v", err)
	}
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "emby-seed", "test", now); err != nil {
		t.Fatalf("AdjustBalance() error = %v", err)
	}
	rating := int32(13)
	input := domain.QueueSetupInput{ID: "emby-account", UserID: user.ID, BaseUsername: "ada", PasswordCiphertext: "sealed-1",
		PasswordContext: "emby.provisioning.password:" + user.ID, SetupPriceTXBMinor: 275,
		Preferences: domain.Preferences{MaxParentalRating: &rating, DisabledLibraryIDs: []string{"movies", "shows"}}}
	account, created, err := store.QueueEmbySetup(ctx, input, now.Add(time.Second))
	if err != nil || !created || account.SetupAttempt != 1 {
		t.Fatalf("QueueEmbySetup() = (%+v, %v, %v)", account, created, err)
	}
	assertBalance(t, store, user.ID, "725")
	if _, created, err := store.QueueEmbySetup(ctx, input, now.Add(2*time.Second)); err != nil || created {
		t.Fatalf("QueueEmbySetup(replay) = (created=%v, err=%v)", created, err)
	}
	assertBalance(t, store, user.ID, "725")
	if _, err := store.FailAndRefundEmbySetup(ctx, account.ID, "terminal provider rejection", now.Add(3*time.Second)); err != nil {
		t.Fatalf("FailAndRefundEmbySetup() error = %v", err)
	}
	if _, err := store.FailAndRefundEmbySetup(ctx, account.ID, "replay", now.Add(4*time.Second)); err != nil {
		t.Fatalf("FailAndRefundEmbySetup(replay) error = %v", err)
	}
	assertBalance(t, store, user.ID, "1000")
	record, err := store.EmbyProvisioningByID(ctx, account.ID)
	if err != nil || record.PasswordCiphertext != "" || record.RefundedAt == nil {
		t.Fatalf("failed record = (%+v, %v)", record, err)
	}
	input.ID = "ignored-new-id"
	input.PasswordCiphertext = "sealed-2"
	retried, created, err := store.QueueEmbySetup(ctx, input, now.Add(5*time.Second))
	if err != nil || !created || retried.ID != account.ID || retried.SetupAttempt != 2 {
		t.Fatalf("QueueEmbySetup(retry) = (%+v, %v, %v)", retried, created, err)
	}
	assertBalance(t, store, user.ID, "725")
	var debitCount, refundCount, outboxCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='emby_setup_debit'`).Scan(&debitCount); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='emby_setup_refund'`).Scan(&refundCount); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND json_extract(payload,'$.accountId')=?`, domain.ProvisionOutboxKind, account.ID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if debitCount != 2 || refundCount != 1 || outboxCount != 2 {
		t.Fatalf("rows debit=%d refund=%d outbox=%d", debitCount, refundCount, outboxCount)
	}
}

func TestEmbyProvisioningTransitionsEraseSecretOnlyAfterSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newEmbyTestStore(t)
	user := createEmbyTestUser(t, store, 31002)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET username='river',onboarding_state='complete' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 8, 11, 0, 0, 0, time.UTC)
	input := domain.QueueSetupInput{ID: "emby-transition", UserID: user.ID, BaseUsername: "river", PasswordCiphertext: "sealed",
		PasswordContext: "emby.provisioning.password:" + user.ID, Preferences: domain.Preferences{DisabledLibraryIDs: []string{"movies"}}}
	account, _, err := store.QueueEmbySetup(ctx, input, now)
	if err != nil {
		t.Fatalf("QueueEmbySetup() error = %v", err)
	}
	if _, err := store.BeginEmbyProvisioning(ctx, account.ID, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if username, err := store.EmbyBaseUsername(ctx, user.ID); err != nil || username != "river" {
		t.Fatalf("EmbyBaseUsername() = (%q, %v)", username, err)
	}
	if err := store.SetEmbyCandidate(ctx, account.ID, "river", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEmbyCandidate(ctx, account.ID, "river", now.Add(2*time.Second)); err != nil {
		t.Fatalf("SetEmbyCandidate(replay) error = %v", err)
	}
	if err := store.MarkEmbyCreateAttempted(ctx, account.ID, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.RequeueEmbyProvisioning(ctx, account.ID, errors.New("temporary outage"), now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	requeued, err := store.EmbyProvisioningByID(ctx, account.ID)
	if err != nil || requeued.Status != domain.StatusQueued || requeued.LastError != "temporary outage" || requeued.PasswordCiphertext != "sealed" || !requeued.Retryable {
		t.Fatalf("requeued record = (%+v, %v)", requeued, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='failed' WHERE kind=? AND json_extract(payload,'$.accountId')=?`, domain.ProvisionOutboxKind, account.ID); err != nil {
		t.Fatal(err)
	}
	if retried, err := store.RetryEmbyProvisioning(ctx, account.ID, now.Add(3*time.Second)); err != nil || !retried.Retryable {
		t.Fatalf("RetryEmbyProvisioning() = (%+v, %v)", retried, err)
	}
	if _, err := store.BeginEmbyProvisioning(ctx, account.ID, now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEmbyRemoteIdentity(ctx, account.ID, "remote-id", "river", now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEmbyRemoteIdentity(ctx, account.ID, "remote-id", "river", now.Add(4*time.Second)); err != nil {
		t.Fatalf("SetEmbyRemoteIdentity(replay) error = %v", err)
	}
	before, err := store.EmbyProvisioningByID(ctx, account.ID)
	if err != nil || before.PasswordCiphertext != "sealed" {
		t.Fatalf("before success = (%+v, %v)", before, err)
	}
	if err := store.MarkEmbyProvisioned(ctx, account.ID, account.Preferences, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	after, err := store.EmbyProvisioningByID(ctx, account.ID)
	if err != nil || after.Status != domain.StatusActive || after.PasswordCiphertext != "" || after.PasswordContext != "" || after.ProvisionedAt == nil {
		t.Fatalf("after success = (%+v, %v)", after, err)
	}
	rating := int32(7)
	updated, err := store.UpdateEmbyPreferences(ctx, account.ID, domain.Preferences{MaxParentalRating: &rating, DisabledLibraryIDs: []string{"shows"}}, now.Add(6*time.Second))
	if err != nil || updated.Preferences.MaxParentalRating == nil || *updated.Preferences.MaxParentalRating != 7 || len(updated.Preferences.DisabledLibraryIDs) != 1 || updated.Preferences.DisabledLibraryIDs[0] != "shows" {
		t.Fatalf("UpdateEmbyPreferences() = (%+v, %v)", updated, err)
	}
	if err := store.TouchEmbyAccount(ctx, account.ID, now.Add(7*time.Second)); err != nil {
		t.Fatalf("TouchEmbyAccount() error = %v", err)
	}
	accounts, err := store.ListEmbyAccounts(ctx, 10)
	if err != nil || len(accounts) != 1 || accounts[0].Retryable {
		t.Fatalf("ListEmbyAccounts() = (%+v, %v)", accounts, err)
	}
}

func assertBalance(t *testing.T, store *Store, userID, want string) {
	t.Helper()
	balance, err := store.Balance(context.Background(), userID)
	if err != nil || balance.Minor != want {
		t.Fatalf("Balance() = (%+v, %v), want %s", balance, err, want)
	}
}

func newEmbyTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(openTestDatabase(t, "emby-test.db"))
}

func createEmbyTestUser(t *testing.T, store *Store, telegramID int64) model.User {
	t.Helper()
	user, created, err := store.UpsertTelegramUser(context.Background(), model.TelegramProfile{
		ID: telegramID, FirstName: "Emby", Username: fmt.Sprintf("emby%d", telegramID),
	}, false)
	if err != nil || !created {
		t.Fatalf("UpsertTelegramUser() = (%+v, %v, %v)", user, created, err)
	}
	return user
}

package database

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

func TestEmbyRefundRejectsChangedStageAndLaterSetupAttempt(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 98_910)
	now := time.Now().UTC()
	if _, err := store.AdjustBalance(ctx, owner.ID, 1000, "seed", "test", now); err != nil {
		t.Fatal(err)
	}
	input := domain.QueueSetupInput{ID: "refund-cas", UserID: owner.ID, BaseUsername: owner.AccountName(),
		SetupPriceTXBMinor: 275, PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:" + owner.ID}
	account, _, err := store.QueueEmbySetup(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := store.BeginEmbyProvisioning(ctx, account.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetEmbyCandidate(ctx, account.ID, input.BaseUsername, now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkEmbyCreateAttempted(ctx, account.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEmbyRemoteIdentity(ctx, account.ID, "remote", input.BaseUsername, now); err != nil {
		t.Fatal(err)
	}
	current, err := store.EmbyProvisioningByID(ctx, account.ID)
	if err != nil || !current.UpdatedAt.Equal(expected.UpdatedAt) {
		t.Fatal("fixture must exercise changed stage even at the same clock", err)
	}
	if _, err := store.FailAndRefundEmbySetupIfUnchanged(ctx, expected, "stale pre-create failure", now); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale failure refunded a created account: %v", err)
	}
	stale := current
	stale.UpdatedAt = stale.UpdatedAt.Add(-time.Nanosecond)
	if _, err := store.FailAndRefundEmbySetupIfUnchanged(ctx, stale, "stale absence observation", now); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	assertBalance(t, store, owner.ID, "725")
	refunded, err := store.FailAndRefundEmbySetupIfUnchanged(ctx, current, "confirmed remote absence", now)
	if err != nil || refunded.Status != domain.StatusFailed || refunded.RefundedAt == nil {
		t.Fatalf("current proof did not complete: %+v, %v", refunded, err)
	}
	replayed, err := store.FailAndRefundEmbySetupIfUnchanged(ctx, current, "retry after response loss", now.Add(time.Second))
	if err != nil || !replayed.UpdatedAt.Equal(refunded.UpdatedAt) {
		t.Fatalf("same-attempt reversal did not replay: %+v, %v", replayed, err)
	}
	assertBalance(t, store, owner.ID, "1000")
	input.PasswordCiphertext = "next-attempt"
	if _, created, err := store.QueueEmbySetup(ctx, input, now); err != nil || !created {
		t.Fatalf("next setup: created=%v error=%v", created, err)
	}
	if _, err := store.FailAndRefundEmbySetupIfUnchanged(ctx, current, "old attempt", now); !errors.Is(err, ErrConflict) {
		t.Fatalf("old failure reversed a new setup: %v", err)
	}
	assertBalance(t, store, owner.ID, "725")
	retained, err := store.EmbyProvisioningByID(ctx, account.ID)
	if err != nil || retained.SetupAttempt != 2 || retained.PasswordCiphertext != "next-attempt" || retained.RefundedAt != nil {
		t.Fatal("new setup was changed by stale refund", err)
	}
	var refunds int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1 AND kind='emby_setup_refund'`, owner.ID).Scan(&refunds); err != nil || refunds != 1 {
		t.Fatalf("refund evidence changed: count=%d error=%v", refunds, err)
	}
}

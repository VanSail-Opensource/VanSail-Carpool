package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAutomaticRenewalAdmissionEndsAtExactExpiry(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, store, 49011)
	combo := saveTestCombo(t, store, "admission", 100, 30)
	at := time.Date(2026, 10, 1, 0, 0, 0, 123, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "admission-seed", "seed", at); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "source"}, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, at); err != nil {
		t.Fatal(err)
	}
	input := PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "manual-successor"}
	before := source.ValidUntil.Add(-time.Nanosecond)
	if _, err := store.QuotePurchase(ctx, input, before); !errors.Is(err, ErrAutoRenewalEnabled) {
		t.Fatal("quote bypassed enabled current term", err)
	}
	if _, err := store.CreatePurchase(ctx, input, before); !errors.Is(err, ErrAutoRenewalEnabled) {
		t.Fatal("transaction bypassed enabled current term", err)
	}
	if enabled, err := store.HasEnabledAutoRenewal(ctx, user.ID, source.ValidUntil); err != nil || enabled {
		t.Fatal("expired source still blocked checkout", enabled, err)
	}
	manual, err := store.CreatePurchase(ctx, input, source.ValidUntil)
	if err != nil {
		t.Fatal("manual purchase after expiry", err)
	}
	recordAutomaticRollover(t, store, source.ID, source.ValidUntil, 1000, 0)
	if _, err := store.CommitAutoRenewal(ctx, source.ID, source.ValidUntil); !errors.Is(err, ErrConflict) {
		t.Fatal("late automatic renewal duplicated the manual successor", err)
	}
	replayed, err := store.CreatePurchase(ctx, input, source.ValidUntil)
	if err != nil || replayed.ID != manual.ID {
		t.Fatal("manual checkout replay changed purchase", replayed, err)
	}
}

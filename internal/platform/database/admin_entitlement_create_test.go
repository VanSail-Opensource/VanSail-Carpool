package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const manualGrantSquad = "55555555-5555-4555-8555-555555555555"

func manualGrantFixture(t *testing.T) (*Store, model.User, model.User, model.Combo, time.Time) {
	t.Helper()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	actor := createTestUser(t, store, 42_001)
	user := createTestUser(t, store, 42_002)
	if err := store.SaveXNodeGroup(ctx, xnode.Group{ID: manualGrantSquad, Name: "manual-grant", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "manual-grant-combo", 9_900, 30, manualGrantSquad)
	return store, actor, user, combo, now
}

func manualGrantInput(actor, user, combo string, now time.Time) AdminEntitlementCreateInput {
	return AdminEntitlementCreateInput{ActorUserID: actor, UserID: user, IdempotencyKey: "manual-grant",
		RequestFingerprint: strings.Repeat("a", 64), Reason: "manual fixed-term grant", ComboID: combo,
		ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(30 * 24 * time.Hour), TrafficLimitBytes: 4_096,
		ResetStrategy: "DAY", SquadUUIDs: []string{manualGrantSquad}}
}

func TestCreateAdminEntitlementGrantsWithoutDebitOrLedgerDebt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, actor, user, combo, now := manualGrantFixture(t)
	before := adminWorkflowBalance(t, store, user.ID)
	input := manualGrantInput(actor.ID, user.ID, combo.ID, now)

	purchase, err := store.CreateAdminEntitlement(ctx, input, now)
	if err != nil {
		t.Fatalf("CreateAdminEntitlement(): %v", err)
	}
	if purchase.UserID != user.ID || purchase.ComboID != combo.ID {
		t.Fatalf("purchase = %+v", purchase)
	}
	if purchase.PriceTXBMinor != 0 || purchase.GrossPriceTXBMinor != 0 || purchase.CoreGrossTXBMinor != 0 ||
		purchase.CouponDiscountTXBMinor != 0 || purchase.Status != "active" {
		t.Fatalf("manual grant facts = %+v", purchase)
	}
	if purchase.TrafficLimitBytes != input.TrafficLimitBytes || purchase.ResetStrategy != input.ResetStrategy ||
		!equalStrings(purchase.SquadUUIDs, input.SquadUUIDs) {
		t.Fatalf("entitlement snapshot = %+v", purchase)
	}
	if purchase.PurchasedTermSeconds == nil || *purchase.PurchasedTermSeconds <= 0 {
		t.Fatalf("purchased term = %v", purchase.PurchasedTermSeconds)
	}
	if adminWorkflowBalance(t, store, user.ID) != before || adminWorkflowLedgerCount(t, store, user.ID, "purchase_debit") != 0 {
		t.Fatal("manual grant moved TXB or created ledger debt")
	}
	var action string
	if err := store.DB().QueryRowContext(ctx, `SELECT action FROM audit_events WHERE target_id=$1`, purchase.ID).Scan(&action); err != nil || action != "entitlement.create" {
		t.Fatalf("audit action = %q, %v", action, err)
	}
	var syncJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user'
		AND payload::jsonb->>'userId'=$1`, user.ID).Scan(&syncJobs); err != nil || syncJobs != 1 {
		t.Fatalf("user sync jobs = %d, %v", syncJobs, err)
	}
	// The member notice is gated on the provider sync and is not yet queued.
	assertNotificationCounts(t, store, 1, 0)
	if err := store.ReleaseUserSyncNotifications(ctx, user.ID, now); err != nil {
		t.Fatalf("ReleaseUserSyncNotifications(): %v", err)
	}
	assertNotificationCounts(t, store, 1, 1)
}

func TestCreateAdminEntitlementIsIdempotentByActorOwnerKeyAndFingerprint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, actor, user, combo, now := manualGrantFixture(t)
	input := manualGrantInput(actor.ID, user.ID, combo.ID, now)
	first, err := store.CreateAdminEntitlement(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateAdminEntitlement(ctx, input, now.Add(time.Minute))
	if err != nil || second.ID != first.ID {
		t.Fatalf("replay = %+v, %v", second, err)
	}
	var purchases, audits, syncJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM purchases`).Scan(&purchases); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='entitlement.create'`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user'
		AND payload::jsonb->>'userId'=$1`, user.ID).Scan(&syncJobs); err != nil {
		t.Fatal(err)
	}
	if purchases != 1 || audits != 1 || syncJobs != 1 {
		t.Fatalf("replay duplicated work: purchases=%d audits=%d sync=%d", purchases, audits, syncJobs)
	}
	input.RequestFingerprint = strings.Repeat("b", 64)
	if _, err := store.CreateAdminEntitlement(ctx, input, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed fingerprint replay = %v", err)
	}
}

func TestCreateAdminEntitlementQueuesFutureTermAndRejectsInvalidTargets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, actor, user, combo, now := manualGrantFixture(t)
	future := manualGrantInput(actor.ID, user.ID, combo.ID, now)
	future.ValidFrom, future.ValidUntil = now.Add(48*time.Hour), now.Add(78*time.Hour)
	queued, err := store.CreateAdminEntitlement(ctx, future, now)
	if err != nil || queued.Status != "queued" {
		t.Fatalf("future grant = %+v, %v", queued, err)
	}
	overlap := manualGrantInput(actor.ID, user.ID, combo.ID, now)
	overlap.IdempotencyKey, overlap.RequestFingerprint = "overlap-grant", strings.Repeat("c", 64)
	if _, err := store.CreateAdminEntitlement(ctx, overlap, now); !errors.Is(err, ErrEntitlementOverlap) {
		t.Fatalf("overlapping grant = %v", err)
	}
	inactive, err := store.SaveCombo(ctx, ComboInput{Name: "inactive", PriceTXBMinor: 100, ValidityDays: 30,
		TrafficLimitBytes: 1_024, ResetStrategy: "DAY", Active: false, SquadProductIDs: []string{manualGrantSquad}})
	if err != nil {
		t.Fatal(err)
	}
	onInactive := manualGrantInput(actor.ID, user.ID, inactive.ID, now)
	onInactive.IdempotencyKey, onInactive.RequestFingerprint = "inactive-grant", strings.Repeat("d", 64)
	if _, err := store.CreateAdminEntitlement(ctx, onInactive, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("inactive combo grant = %v", err)
	}
	missingSquad := manualGrantInput(actor.ID, user.ID, combo.ID, now)
	missingSquad.IdempotencyKey, missingSquad.RequestFingerprint = "missing-squad", strings.Repeat("e", 64)
	missingSquad.SquadUUIDs = []string{"66666666-6666-4666-8666-666666666666"}
	if _, err := store.CreateAdminEntitlement(ctx, missingSquad, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing squad grant = %v", err)
	}
}

func TestCreateAdminEntitlementRejectsActiveHourlyEnrollment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, actor, user, combo, now := manualGrantFixture(t)
	insertManualHourlyEnrollment(t, store, user.ID, user.AccountGeneration, manualGrantSquad, "preparing")
	if _, err := store.CreateAdminEntitlement(ctx, manualGrantInput(actor.ID, user.ID, combo.ID, now), now); !errors.Is(err, ErrHourlyLegacyConflict) {
		t.Fatalf("grant over active hourly enrollment = %v", err)
	}
	var purchases int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM purchases`).Scan(&purchases); err != nil || purchases != 0 {
		t.Fatalf("hourly-conflicting grant persisted a purchase: %d, %v", purchases, err)
	}
}

func TestCreateAdminEntitlementIgnoresTerminatedHourlyEnrollment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, actor, user, combo, now := manualGrantFixture(t)
	insertManualHourlyEnrollment(t, store, user.ID, user.AccountGeneration, manualGrantSquad, "disabled")
	purchase, err := store.CreateAdminEntitlement(ctx, manualGrantInput(actor.ID, user.ID, combo.ID, now), now)
	if err != nil || purchase.Status != "active" {
		t.Fatalf("grant over terminated hourly enrollment = %+v, %v", purchase, err)
	}
}

func TestCreateAdminEntitlementAllowsAdjacentSuccessorAndRejectsOverlap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, actor, user, combo, now := manualGrantFixture(t)
	active, err := store.CreateAdminEntitlement(ctx, manualGrantInput(actor.ID, user.ID, combo.ID, now), now)
	if err != nil {
		t.Fatalf("initial grant = %v", err)
	}
	adjacent := manualGrantInput(actor.ID, user.ID, combo.ID, now)
	adjacent.IdempotencyKey, adjacent.RequestFingerprint = "adjacent-term", strings.Repeat("f", 64)
	adjacent.Reason, adjacent.ValidFrom, adjacent.ValidUntil = "adjacent successor", active.ValidUntil, active.ValidUntil.Add(24*time.Hour)
	successor, err := store.CreateAdminEntitlement(ctx, adjacent, now)
	if err != nil {
		t.Fatalf("adjacent successor = %v", err)
	}
	if successor.Status != "queued" || !successor.ValidFrom.Equal(active.ValidUntil) {
		t.Fatalf("adjacent successor = %+v", successor)
	}
	overlap := adjacent
	overlap.IdempotencyKey, overlap.RequestFingerprint = "overlap-term", strings.Repeat("b", 64)
	overlap.ValidFrom, overlap.ValidUntil = active.ValidUntil.Add(-time.Second), active.ValidUntil.Add(24*time.Hour)
	if _, err := store.CreateAdminEntitlement(ctx, overlap, now); !errors.Is(err, ErrEntitlementOverlap) {
		t.Fatalf("overlapping successor = %v", err)
	}
}

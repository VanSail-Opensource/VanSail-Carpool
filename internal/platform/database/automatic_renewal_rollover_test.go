package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func recordAutomaticRollover(t *testing.T, store *Store, purchaseID string, now time.Time, used, eligible int64) {
	t.Helper()
	if err := store.EnqueueDueEntitlementTransitions(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkRolloverProcessing(context.Background(), purchaseID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordRolloverCalculation(context.Background(), purchaseID, model.RolloverUsageSummary{
		AllocatedBytes: 1_000, UsedBytes: used, EligibleUnusedBytes: eligible, AlgorithmVersion: "cadence-v4",
	}, now); err != nil {
		t.Fatal(err)
	}
}

func TestCalculatedRolloverFundsAutomaticRenewalAtomically(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_010)
	combo := saveTestCombo(t, store, "atomic-rollover", 100, 1)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 160, "atomic-rollover-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "atomic-rollover-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
		t.Fatalf("enable automatic renewal: %v", err)
	}
	recordAutomaticRollover(t, store, source.ID, source.ValidUntil, 500, 500)
	successor, err := store.CommitAutoRenewal(ctx, source.ID, source.ValidUntil)
	if err != nil || successor.Status != "activating" {
		t.Fatalf("CommitAutoRenewal() = (%+v, %v)", successor, err)
	}
	if balance, err := store.Balance(ctx, user.ID); err != nil || balance.MinorInt64() != 10 {
		t.Fatalf("Balance() = (%+v, %v), want 10", balance, err)
	}
	if replay, err := store.CommitAutoRenewal(ctx, source.ID, source.ValidUntil); err != nil || replay.ID != successor.ID {
		t.Fatalf("CommitAutoRenewal(replay) = (%+v, %v)", replay, err)
	}
	var credits, debits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='rollover_credit'`).Scan(&credits); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='automatic_renewal'`).Scan(&debits); err != nil {
		t.Fatal(err)
	}
	if credits != 1 || debits != 1 {
		t.Fatalf("credit/debit ledgers = %d/%d, want 1/1", credits, debits)
	}
}

func TestLegacyCalculatedRolloverIsCorrectedBeforeCredit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_019)
	combo, err := store.SaveCombo(ctx, ComboInput{
		Name: "whole-term-rollover", PriceTXBMinor: 58_800, ValidityDays: 1,
		TrafficLimitBytes: 53_472, ResetStrategy: "DAY", Active: true, RolloverMinRemainingBPS: 8_500,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 117_600, "rollover-correction-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "rollover-correction"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkRolloverProcessing(ctx, source.ID, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordRolloverCalculation(ctx, source.ID, model.RolloverUsageSummary{
		AllocatedBytes: 53_472, UsedBytes: 7_518, EligibleUnusedBytes: 6_294, AlgorithmVersion: "cadence-v3",
	}, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchase_rollovers SET allocated_traffic_bytes=53472,
		used_traffic_bytes=7518,eligible_unused_bytes=6294,algorithm_version='cadence-v3' WHERE purchase_id=?`, source.ID); err != nil {
		t.Fatal(err)
	}
	successor, err := store.CommitAutoRenewal(ctx, source.ID, source.ValidUntil)
	if err != nil {
		t.Fatal(err)
	}
	rollover, err := store.RolloverByPurchase(ctx, source.ID)
	if err != nil || rollover.EligibleUnusedBytes == nil || *rollover.EligibleUnusedBytes != 45_954 ||
		rollover.CreditedTXBMinor != 50_533 || rollover.AlgorithmVersion != "cadence-v4" {
		t.Fatalf("corrected rollover = %+v, err %v", rollover, err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, successor.ID, true, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	var eligible, credited string
	if err := store.DB().QueryRowContext(ctx, `SELECT json_extract(payload_json,'$.facts.eligibleBytes'),
		json_extract(payload_json,'$.facts.rolloverMinor') FROM user_notification_events WHERE event_key=?`,
		"auto-renewal:"+successor.ID).Scan(&eligible, &credited); err != nil || eligible != "45954" || credited != "50533" {
		t.Fatalf("renewal notice eligible/credit = %q/%q, err %v", eligible, credited, err)
	}
}

func TestCalculatedRolloverCanBeExcludedFromAutomaticRenewalBalance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_012)
	combo := saveTestCombo(t, store, "rollover-after-renewal", 100, 1)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 160, "rollover-after-renewal-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "rollover-after-renewal-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
		t.Fatal(err)
	}
	recordAutomaticRollover(t, store, source.ID, source.ValidUntil, 500, 500)
	if _, err := store.CommitAutoRenewalWithRolloverBalance(ctx, source.ID, false, source.ValidUntil); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("CommitAutoRenewalWithRolloverBalance() = %v, want ErrInsufficientBalance", err)
	}
	if balance, err := store.Balance(ctx, user.ID); err != nil || balance.MinorInt64() != 60 {
		t.Fatalf("Balance() = (%+v, %v), want 60", balance, err)
	}
	var credits, debits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='rollover_credit'`).Scan(&credits); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='automatic_renewal'`).Scan(&debits); err != nil {
		t.Fatal(err)
	}
	if credits != 0 || debits != 0 {
		t.Fatalf("credit/debit ledgers = %d/%d, want 0/0", credits, debits)
	}
}

func TestRolloverEligibilityRequiresAutoRenewalWithoutQueuedCombo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_011)
	combo := saveTestCombo(t, store, "rollover-gate", 100, 1)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 400, "rollover-gate-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "rollover-gate-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if eligible, err := store.RolloverEligible(ctx, source.ID); err != nil || eligible {
		t.Fatalf("RolloverEligible(auto off) = (%t, %v)", eligible, err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if eligible, err := store.RolloverEligible(ctx, source.ID); err != nil || !eligible {
		t.Fatalf("RolloverEligible(auto on) = (%t, %v)", eligible, err)
	}
	// Checkout now enforces admission inside the transaction. Seed the legacy
	// queued/auto-enabled combination explicitly through separate preference steps.
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, false, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "rollover-gate-queued"}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if eligible, err := store.RolloverEligible(ctx, source.ID); err != nil || eligible {
		t.Fatalf("RolloverEligible(queued combo) = (%t, %v)", eligible, err)
	}
}

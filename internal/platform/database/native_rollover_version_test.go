package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

func TestNativeRolloverProvenanceSurvivesAtomicRenewal(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, store, 48029)
	combo := saveTestCombo(t, store, "native-rollover", 100, 1)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 160, "native-rollover-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "native-rollover"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if err = store.EnqueueDueEntitlementTransitions(ctx, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	if err = store.MarkRolloverProcessing(ctx, source.ID, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	calculated, err := store.RecordRolloverCalculation(ctx, source.ID, model.RolloverUsageSummary{AllocatedBytes: 1000, UsedBytes: 500, EligibleUnusedBytes: 500, AlgorithmVersion: rollover.NativeUsageAlgorithmVersion}, source.ValidUntil)
	if err != nil || calculated.AlgorithmVersion != rollover.NativeUsageAlgorithmVersion {
		t.Fatalf("native calculation provenance lost: %+v,%v", calculated, err)
	}
	if _, err = store.CommitAutoRenewal(ctx, source.ID, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	settled, err := store.RolloverByPurchase(ctx, source.ID)
	if err != nil || settled.AlgorithmVersion != rollover.NativeUsageAlgorithmVersion || settled.CreditedTXBMinor != 50 {
		t.Fatalf("native settlement changed: %+v,%v", settled, err)
	}
}

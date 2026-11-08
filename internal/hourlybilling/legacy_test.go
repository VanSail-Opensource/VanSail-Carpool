package hourlybilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestLegacyAdmissionConflictTerminatesWithoutDebitOrNativePreparation(t *testing.T) {
	for _, status := range []string{"active", "queued"} {
		t.Run(status, func(t *testing.T) {
			s, user, policy, native := commerceFixture(t)
			ctx := context.Background()
			combo, err := s.store.SaveCombo(ctx, database.ComboInput{Name: "Grandfathered", Active: true, ValidityDays: 30, PriceTXBMinor: 0, TrafficLimitBytes: 1000, ResetStrategy: "NO_RESET"})
			if err != nil {
				t.Fatal(err)
			}
			purchase, err := s.store.CreatePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "legacy"}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.store.DB().Exec(`UPDATE purchases SET status=$1 WHERE id=$2`, status, purchase.ID); err != nil {
				t.Fatal(err)
			}
			op, job := queuedActivation(t, s, user, policy)
			if err = s.HandleProviderOperation(ctx, op, job); err != nil {
				t.Fatal(err)
			}
			op, _ = reloadOperation(t, s, op.Receipt.ID)
			if op.Receipt.Status != "failed" || op.Receipt.ErrorCode == nil || *op.Receipt.ErrorCode != "HOURLY_LEGACY_ACTIVE" || native.resumes != 0 || native.closes != 0 || balanceFixture(t, s, user) != 100000 {
				t.Fatal("legacy conflict remained retryable or mutated account")
			}
		})
	}
}

func TestDisablePreservesTransientRepositoryFailure(t *testing.T) {
	s, user, policy, _ := commerceFixture(t)
	if err := s.store.DB().Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Disable(context.Background(), user, "unknown-enrollment", policy.snapshot.Version, "disable"); err == nil || errors.Is(err, database.ErrNotFound) {
		t.Fatalf("repository transport failure became not found: %v", err)
	}
}

package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestLatestComboControlKeepsUnresolvedWorkAndHidesSupersededFailures(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	combo := saveTestCombo(t, store, "control-projection", 100, 30)
	user, purchase := createAdminWorkflowPurchase(t, store, 49_011, combo, now)
	create := func(key, status string) providerops.Operation {
		t.Helper()
		operation, _, err := store.CreateProviderOperation(ctx,
			memberOperationInput(user.ID, purchase.ID, purchaseops.OperationSquadSwitch, key), now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET status=$1 WHERE id=$2`, status, operation.Receipt.ID); err != nil {
			t.Fatal(err)
		}
		return operation
	}
	assertLatest := func(want string) {
		t.Helper()
		receipt, err := store.LatestComboControl(ctx, user.ID)
		if err != nil || (want == "" && receipt != nil) || (want != "" && (receipt == nil || receipt.ID != want)) {
			t.Fatalf("latest = %+v, %v; want %q", receipt, err, want)
		}
	}

	failed := create("failed-control", "failed")
	assertLatest(failed.Receipt.ID)
	create("superseding-control", "succeeded")
	assertLatest("")
	retained, err := store.ProviderOperationByID(ctx, failed.Receipt.ID)
	if err != nil || retained.Receipt.Status != "failed" {
		t.Fatalf("hidden failure lost its durable receipt: %+v, %v", retained, err)
	}

	pending := create("unresolved-control", "pending_review")
	create("later-failed-control", "failed")
	assertLatest(pending.Receipt.ID)
	create("later-successful-control", "succeeded")
	assertLatest(pending.Receipt.ID)
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET status='compensated' WHERE id=$1`, pending.Receipt.ID); err != nil {
		t.Fatal(err)
	}
	assertLatest("")
	create("current-failed-control", "failed")
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, stamp(now.Add(-time.Minute)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	assertLatest("")
}

func TestLiveComboControlUsesInstantsAtFractionalBoundaries(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2030, 10, 8, 0, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "control-instants", 100, 30)
	user, purchase := createAdminWorkflowPurchase(t, store, 49_012, combo, now)
	operation, _, err := store.CreateProviderOperation(ctx,
		memberOperationInput(user.ID, purchase.ID, purchaseops.OperationEarlyActivation, "instant-control"), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name             string
		from, until, at  time.Time
		want             bool
	}{
		{name: "future fractional start", from: now.Add(500 * time.Millisecond), until: now.Add(time.Hour), at: now},
		{name: "future fractional expiry", from: now.Add(-time.Hour), until: now.Add(500 * time.Millisecond), at: now, want: true},
		{name: "whole-second start before fractional read", from: now, until: now.Add(time.Hour), at: now.Add(500 * time.Millisecond), want: true},
		{name: "exact expiry", from: now.Add(-time.Hour), until: now, at: now},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`,
				stamp(test.from), stamp(test.until), purchase.ID); err != nil {
				t.Fatal(err)
			}
			var live bool
			if err := store.DB().QueryRowContext(ctx, `SELECT (`+liveComboControlPredicate(2)+`)
 FROM provider_operations operation WHERE operation.id=$1`, operation.Receipt.ID,
				stamp(test.at), stamp(test.at), stamp(test.at), stamp(test.at)).Scan(&live); err != nil || live != test.want {
				t.Fatalf("live control = %v, %v; want %v", live, err, test.want)
			}
		})
	}
}

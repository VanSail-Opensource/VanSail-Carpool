package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func memberOperationInput(userID, purchaseID, kind, key string) providerops.CreateInput {
	return providerops.CreateInput{ActorUserID: userID, OwnerUserID: userID, Kind: kind, IdempotencyKey: key,
		RequestFingerprint: fmt.Sprintf("%x", sha256.Sum256([]byte(kind+":"+purchaseID+":"+key))),
		Items:              []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: purchaseID}}}
}

func TestSquadSwitchPreservesOwnershipAndLastEnabled(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2030, 10, 8, 0, 0, 0, 0, time.UTC)
	saveTestSquad(t, store, "squad-a", 0, true)
	saveTestSquad(t, store, "squad-b", 0, true)
	combo := saveTestCombo(t, store, "switch", 1000, 30, "squad-a", "squad-b")
	user, purchase := createAdminWorkflowPurchase(t, store, 49001, combo, now)
	balance := adminWorkflowBalance(t, store, user.ID)
	input := memberOperationInput(user.ID, purchase.ID, purchaseops.OperationSquadSwitch, "switch-a")
	operation, err := store.BeginSquadSwitch(ctx, input, purchase.ID, "squad-a", false, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := store.BeginSquadSwitch(ctx, input, purchase.ID, "squad-a", false, now)
	if err != nil || replay.Receipt.ID != operation.Receipt.ID {
		t.Fatalf("replay = %v, %v", replay, err)
	}
	completeComboControlFixture(t, store, operation, now)
	_, err = store.BeginSquadSwitch(ctx, memberOperationInput(user.ID, purchase.ID, purchaseops.OperationSquadSwitch, "switch-b"), purchase.ID, "squad-b", false, now)
	if !errors.Is(err, purchaseops.ErrLastEnabledSquad) {
		t.Fatalf("last squad error = %v", err)
	}
	after, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || !slices.Equal(after.SquadUUIDs, purchase.SquadUUIDs) || adminWorkflowBalance(t, store, user.ID) != balance {
		t.Fatalf("ownership or balance changed: %+v, %v", after, err)
	}
	_, err = store.BeginSquadSwitch(ctx, memberOperationInput(user.ID, purchase.ID, purchaseops.OperationSquadSwitch, "unowned"), purchase.ID, "foreign-squad", true, now)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("unowned squad error = %v", err)
	}
}

func TestConcurrentSquadSwitchesKeepAccess(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2030, 10, 8, 0, 0, 0, 0, time.UTC)
	saveTestSquad(t, store, "squad-a", 0, true)
	saveTestSquad(t, store, "squad-b", 0, true)
	combo := saveTestCombo(t, store, "concurrent-switch", 1000, 30, "squad-a", "squad-b")
	user, purchase := createAdminWorkflowPurchase(t, store, 49002, combo, now)
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, uuid := range purchase.SquadUUIDs {
		wait.Go(func() {
			_, err := store.BeginSquadSwitch(ctx, memberOperationInput(user.ID, purchase.ID, purchaseops.OperationSquadSwitch, "switch:"+uuid), purchase.ID, uuid, false, now)
			results <- err
		})
	}
	wait.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		} else if !errors.Is(err, ErrConflict) && !errors.Is(err, purchaseops.ErrLastEnabledSquad) {
			t.Fatal(err)
		}
	}
	disabled, err := store.UserDisabledSquads(ctx, user.ID)
	if err != nil || succeeded != 1 || len(purchaseops.EnabledSquads(purchase.SquadUUIDs, disabled)) != 1 {
		t.Fatalf("successes=%d disabled=%v error=%v", succeeded, disabled, err)
	}
}

func TestSquadPreferencesCarryAndRestoreStableFirst(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 49003)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET proxy_user_id='49003' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO user_disabled_squads VALUES(?,'squad-a'),(?,'squad-b')`, user.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	enabled, err := store.EnabledSquadsForRemote(ctx, "49003", []string{"squad-b", "squad-a"})
	if err != nil || !slices.Equal(enabled, []string{"squad-a"}) {
		t.Fatalf("fallback=%v, %v", enabled, err)
	}
	disabled, err := store.UserDisabledSquads(ctx, user.ID)
	if err != nil || !slices.Equal(disabled, []string{"squad-b"}) {
		t.Fatalf("preferences=%v, %v", disabled, err)
	}
	enabled, err = store.EnabledSquadsForRemote(ctx, "49003", []string{"squad-b", "squad-c"})
	if err != nil || !slices.Equal(enabled, []string{"squad-c"}) {
		t.Fatalf("carried preferences=%v, %v", enabled, err)
	}
	enabled, err = store.EnabledSquadsForRemote(ctx, "49003", []string{"squad-b"})
	if err != nil || !slices.Equal(enabled, []string{"squad-b"}) {
		t.Fatalf("single squad=%v, %v", enabled, err)
	}
}

func completeComboControlFixture(t *testing.T, store *Store, operation providerops.Operation, now time.Time) {
	t.Helper()
	ctx := context.Background()
	if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "purchase", now); err != nil {
		t.Fatal(err)
	}
	completion := providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: "{}"}
	if _, err := store.CompleteProviderOperationItem(ctx, operation.Receipt.ID, "purchase", completion, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, now); err != nil {
		t.Fatal(err)
	}
}

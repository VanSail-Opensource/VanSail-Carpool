package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestUpdateIntentIsImmutableBeforeAttemptAndSurvivesRestart(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 498033)
	create := func(key string) providerops.Operation {
		t.Helper()
		operation, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: user.ID, Kind: xnode.OperationKind, IdempotencyKey: key, RequestFingerprint: strings.Repeat("a", 64), Items: []providerops.ItemInput{{Key: "node", TargetType: "xnode", TargetID: "node"}}}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return operation
	}
	operation := create("prepared")
	expected := xnode.UpdateExpectation{Command: "update.install", TransactionID: operation.Receipt.ID, Version: "v2.0.18", Previous: "v2.0.17"}
	prepared, err := store.PrepareXNodeUpdate(ctx, operation.Receipt.ID, expected, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	changed := expected
	changed.Version = "v2.0.19"
	other, err := store.PrepareXNodeUpdate(ctx, operation.Receipt.ID, changed, time.Now())
	if err != nil || other.ResultJSON != prepared.ResultJSON {
		t.Fatal("concurrent/latest release change overwrote intent", err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
	intent, ok := xnode.UpdateExpectationFromResult(reloaded.ResultJSON)
	if err != nil || !ok || intent != expected || reloaded.Attempts != 1 {
		t.Fatal("restart lost original intent", reloaded.Attempts, err)
	}
	completed, err := store.CompleteXNodeProviderOperation(ctx, operation.Receipt.ID, 1, providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: prepared.ResultJSON}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := store.PrepareXNodeUpdate(ctx, operation.Receipt.ID, changed, time.Now())
	if err != nil || retained.ResultJSON != completed.ResultJSON || retained.Receipt.Status != "succeeded" {
		t.Fatal("settled receipt was rewritten", err)
	}
	legacy := create("legacy-interrupted")
	if _, err := store.BeginProviderOperationAttempt(ctx, legacy.Receipt.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	expected.TransactionID = legacy.Receipt.ID
	if _, err := store.PrepareXNodeUpdate(ctx, legacy.Receipt.ID, expected, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatal("invented intent after an uncertain mutation", err)
	}
	rollback := create("rollback")
	rollbackIntent := xnode.UpdateExpectation{Command: "update.rollback", TransactionID: "original-install", Version: "v2.0.18", Previous: "v2.0.17"}
	if _, err := store.PrepareXNodeUpdate(ctx, rollback.Receipt.ID, rollbackIntent, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, rollback.Receipt.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	rollback, err = store.ProviderOperationByID(ctx, rollback.Receipt.ID)
	retainedRollback, ok := xnode.UpdateExpectationFromResult(rollback.ResultJSON)
	if err != nil || !ok || retainedRollback != rollbackIntent || retainedRollback.TransactionID == rollback.Receipt.ID {
		t.Fatal("rollback lost the original install transaction", err)
	}
}

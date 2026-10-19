package database

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestSquadControlsIgnoreUnrelatedProviderOperationsAndKeepEntitlementFence(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	saveTestSquad(t, store, "a", 0, true)
	saveTestSquad(t, store, "b", 0, true)
	combo := saveTestCombo(t, store, "native-squads", 1000, 30, "a", "b")
	user, purchase := createAdminWorkflowPurchase(t, store, 89011, combo, now)
	for _, kind := range []string{"ip_lookup", "emby_setup", "payment_create"} {
		if _, _, err := store.CreateProviderOperation(ctx, memberOperationInput(user.ID, purchase.ID, kind, kind), now); err != nil {
			t.Fatal(err)
		}
	}
	// Historical unrelated jobs may carry non-object bodies; distinct payloads
	// preserve the outbox constraints while exercising the entitlement guard.
	if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET payload=json_array(id),status='processing' WHERE kind='provider_operation'`); err != nil {
		t.Fatal(err)
	}
	if conflict, err := store.ComboControlConflict(ctx, user.ID); err != nil || conflict {
		t.Fatal("unrelated work blocked controls", conflict, err)
	}
	op, err := store.BeginSquadSwitch(ctx, memberOperationInput(user.ID, purchase.ID, purchaseops.OperationSquadSwitch, "native-switch"), purchase.ID, "a", false, now)
	if err != nil {
		t.Fatal(err)
	}
	completeComboControlFixture(t, store, op, now)
	if _, _, err := store.CreateProviderOperation(ctx, memberOperationInput(user.ID, purchase.ID, purchaseops.OperationResetKind, "reset"), now); err != nil {
		t.Fatal(err)
	}
	if conflict, err := store.ComboControlConflict(ctx, user.ID); err != nil || !conflict {
		t.Fatal("reset lost its entitlement fence", conflict, err)
	}
	if _, err := store.BeginSquadSwitch(ctx, memberOperationInput(user.ID, purchase.ID, purchaseops.OperationSquadSwitch, "overlap"), purchase.ID, "a", true, now); !errors.Is(err, ErrConflict) {
		t.Fatal("overlapping entitlement mutation accepted", err)
	}
}

func TestNativeSquadPreferencesDoNotNeedLegacyLink(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 89012)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET remna_user_id=NULL WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO user_disabled_squads VALUES(?,'b')`, user.ID); err != nil {
		t.Fatal(err)
	}
	enabled, err := store.EnabledSquadsForUser(ctx, user.ID, []string{"b", "a"})
	if err != nil || !slices.Equal(enabled, []string{"a"}) {
		t.Fatal(enabled, err)
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO user_disabled_squads VALUES(?,'a')`, user.ID); err != nil {
		t.Fatal(err)
	}
	enabled, err = store.EnabledSquadsForUser(ctx, user.ID, []string{"b", "a"})
	if err != nil || !slices.Equal(enabled, []string{"a"}) {
		t.Fatal(enabled, err)
	}
	disabled, err := store.UserDisabledSquads(ctx, user.ID)
	if err != nil || !slices.Equal(disabled, []string{"b"}) {
		t.Fatal("last-enabled preference was not normalized", disabled, err)
	}
}

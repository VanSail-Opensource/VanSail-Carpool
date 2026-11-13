package database

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestAdminResolutionReplayReturnsItsImmutableReceipt(t *testing.T) {
	store, input, now := resolutionFixture(t, providerops.KindAdminBulkExtension, providerops.StatusSucceeded, providerops.StatusFailed)
	input.Resolution = "partial"
	first, err := store.ResolveAdminOperation(t.Context(), input, now)
	if err != nil || first.Status != "partial" {
		t.Fatal("mixed settled items did not substantiate partial result", first, err)
	}
	// A later domain repair must not replace this idempotency key's result.
	if _, err := store.DB().ExecContext(t.Context(), `UPDATE provider_operations SET status='failed',error_code='LATER_OUTCOME',updated_at=$1,completed_at=$1 WHERE id=$2`, stamp(now.Add(time.Minute)), input.OperationID); err != nil {
		t.Fatal(err)
	}
	replayed, err := store.ResolveAdminOperation(t.Context(), input, now.Add(2*time.Minute))
	if err != nil || !reflect.DeepEqual(first, replayed) {
		t.Fatal("old resolution replay substituted a later mutable receipt", first, replayed, err)
	}
	input.RequestFingerprint = "different-request-fingerprint"
	if _, err = store.ResolveAdminOperation(t.Context(), input, now); !errors.Is(err, ErrConflict) {
		t.Fatal("changed resolution intent reused an old key", err)
	}
	events, err := store.ListAuditEvents(t.Context(), 100)
	if err != nil || len(events) != 1 {
		t.Fatal("replay rewrote or duplicated audit evidence", events, err)
	}
}

func TestAdminResolutionAuditFailureRollsBackReceiptAndReplay(t *testing.T) {
	store, input, now := resolutionFixture(t, providerops.KindAdminComboReplacement, providerops.StatusSucceeded)
	for _, query := range []string{
		`CREATE FUNCTION reject_resolution_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$`,
		`CREATE TRIGGER reject_resolution_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.action='provider_operation.resolve') EXECUTE FUNCTION reject_resolution_audit()`,
	} {
		if _, err := store.DB().ExecContext(t.Context(), query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.ResolveAdminOperation(t.Context(), input, now); err == nil {
		t.Fatal("receipt repair committed without audit")
	}
	assertNoResolutionWrites(t, store, input.OperationID)
	if _, err := store.DB().ExecContext(t.Context(), `DROP TRIGGER reject_resolution_audit ON audit_events`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveAdminOperation(t.Context(), input, now); err != nil {
		t.Fatal("same key could not retry after audit recovery", err)
	}
}

func TestLegacyAdminResolutionReplayRejectsChangedReceipt(t *testing.T) {
	store, input, now := resolutionFixture(t, providerops.KindAdminEntitlementEdit, providerops.StatusSucceeded)
	if _, err := store.DB().ExecContext(t.Context(), `UPDATE provider_operations SET status='succeeded',updated_at=$1,completed_at=$1 WHERE id=$2`, stamp(now), input.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(t.Context(), `INSERT INTO provider_operation_replays(id,operation_id,actor_user_id,idempotency_key,request_fingerprint,created_at) VALUES('legacy-resolution',$1,$2,$3,$4,$5)`,
		input.OperationID, input.ActorUserID, input.IdempotencyKey, input.RequestFingerprint, stamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveAdminOperation(t.Context(), input, now.Add(time.Minute)); err != nil {
		t.Fatal("unchanged historical decision lost replay", err)
	}
	if _, err := store.DB().ExecContext(t.Context(), `UPDATE provider_operations SET status='failed',updated_at=$1,completed_at=$1 WHERE id=$2`, stamp(now.Add(time.Minute)), input.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveAdminOperation(t.Context(), input, now.Add(2*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatal("legacy replay returned a different outcome as its original decision", err)
	}
}

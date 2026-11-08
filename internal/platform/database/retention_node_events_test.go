package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestNodeDownRetentionUsesCreationForEveryStatusAndClearsReferences(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-7 * 24 * time.Hour)
	for index, status := range []string{"observing", "pending_review", "queued", "dismissed", "ineligible"} {
		id := status + "-expired"
		if _, err := store.DB().ExecContext(ctx, `INSERT INTO node_compensation_events(id,node_uuid,node_name,status,offline_observed_at,threshold_minutes,multiplier_bps,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,1,10000,$6,$7)`, id, id, id, status, stamp(now.Add(-time.Hour)), stamp(cutoff), stamp(now)); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().ExecContext(ctx, `INSERT INTO node_compensation_event_squads(event_id,squad_uuid,squad_name) VALUES($1,$2,$3)`, id, "squad", "Squad"); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().ExecContext(ctx, `INSERT INTO node_compensation_node_state(node_uuid,node_name,is_connected,is_disabled,open_event_id,last_observed_at,updated_at)
			VALUES($1,$2,$3,0,$4,$5,$6)`, id, id, index%2, id, stamp(now), stamp(now)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO node_compensation_events(id,node_uuid,node_name,status,offline_observed_at,threshold_minutes,multiplier_bps,created_at,updated_at)
		VALUES('retained','retained','Retained','observing',$1,1,10000,$2,$3)`, stamp(cutoff.Add(-24*time.Hour)), stamp(cutoff.Add(time.Nanosecond)), stamp(now)); err != nil {
		t.Fatal(err)
	}
	counts, err := store.CompactAndPrune(ctx, cutoff, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if counts["node_compensation_events"] != 5 {
		t.Fatalf("counts=%+v", counts)
	}
	var events, refs, details, fk int
	for _, query := range []struct {
		sql   string
		count *int
	}{{`SELECT COUNT(*) FROM node_compensation_events`, &events}, {`SELECT COUNT(*) FROM node_compensation_node_state WHERE open_event_id IS NOT NULL`, &refs}, {`SELECT COUNT(*) FROM node_compensation_event_squads`, &details}, {`SELECT COUNT(*) FROM pg_constraint WHERE connamespace='public'::regnamespace AND contype='f' AND NOT convalidated`, &fk}} {
		if err := store.DB().QueryRowContext(ctx, query.sql).Scan(query.count); err != nil {
			t.Fatal(err)
		}
	}
	if events != 1 || refs != 0 || details != 0 || fk != 0 {
		t.Fatalf("events/refs/details/FKs=%d/%d/%d/%d", events, refs, details, fk)
	}
}

func TestExpiredApprovedOutageKeepsExtensionsAndExecutableSync(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	old := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	now := old.Add(8 * 24 * time.Hour)
	combo := saveTestCombo(t, store, "retained-compensation", 100, 30)
	squad := saveTestSquad(t, store, "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", 10, true)
	actor := createTestUser(t, store, 31931)
	member, purchase := createAdminWorkflowPurchase(t, store, 31932, combo, old, squad.GroupID)
	legacy, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: actor.ID, OwnerUserID: member.ID, Kind: providerops.KindLegacyOutageSync,
		IdempotencyKey: "legacy-approved", RequestFingerprint: "legacy-approved-fingerprint",
		Items: []providerops.ItemInput{{Key: member.ID, TargetType: "user", TargetID: member.ID}},
	}, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, stamp(purchase.ValidUntil.Add(17*time.Minute)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `INSERT INTO node_compensation_events(id,node_uuid,node_name,status,offline_observed_at,threshold_minutes,multiplier_bps,frozen_recipient_count,provider_operation_id,revision,created_at,updated_at) VALUES('legacy-approved','legacy-node','Legacy node','queued',$1,1,10000,1,$2,0,$1,$1)`, stamp(old), legacy.Receipt.ID); err != nil {
		t.Fatal(err)
	}
	counts, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if counts["node_compensation_events"] != 1 {
		t.Fatalf("counts=%+v", counts)
	}
	updated, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || !updated.ValidUntil.Equal(purchase.ValidUntil.Add(17*time.Minute)) {
		t.Fatalf("extension changed=%+v,%v", updated, err)
	}
	operation, err := store.ProviderOperationByID(ctx, legacy.Receipt.ID)
	if err != nil || operation.Receipt.Status != string(providerops.StatusQueued) {
		t.Fatalf("sync operation=%+v,%v", operation, err)
	}
	items, err := store.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].TargetType != "user" {
		t.Fatalf("independent targets=%+v,%v", items, err)
	}
	var jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='provider_operation' AND (CASE ((payload)::jsonb #> '{operationId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{operationId}' END)=$1`, operation.Receipt.ID).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("sync job=%d,%v", jobs, err)
	}
}

package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestCompensationApprovalSkipsInactiveLinksOperationAndIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 23, 3, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "compensation-review", 100, 30)
	squad := saveTestSquad(t, store, "99999999-9999-4999-8999-999999999999", 10, true)
	actor := createTestUser(t, store, 49_000)
	_, active := createAdminWorkflowPurchase(t, store, 49_001, combo, now, squad.RemnaSquadUUID)
	_, inactive := createAdminWorkflowPurchase(t, store, 49_002, combo, now, squad.RemnaSquadUUID)
	event := recoveredCompensationEvent(t, store, now, squad.RemnaSquadUUID, squad.Name)
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=? WHERE id=?`, stamp(now), inactive.ID); err != nil {
		t.Fatal(err)
	}
	input := compensation.ReviewInput{EventID: event.ID, ActorUserID: actor.ID, IdempotencyKey: "approve-key",
		Reason: "reviewed outage", Revision: event.Revision, ExtensionMinutes: 17}
	approved, err := store.ApproveCompensationEvent(ctx, input, "approve-fingerprint", now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("ApproveCompensationEvent(): %v", err)
	}
	if approved.Status != "queued" || approved.Operation == nil || approved.Operation.Kind != providerops.KindNodeCompensation ||
		approved.EligibleRecipientCount == nil || *approved.EligibleRecipientCount != 1 ||
		approved.SkippedRecipientCount == nil || *approved.SkippedRecipientCount != 1 {
		t.Fatalf("approved event = %+v", approved)
	}
	shifted, _ := store.PurchaseByID(ctx, active.ID)
	if !shifted.ValidUntil.Equal(active.ValidUntil.Add(17 * time.Minute)) {
		t.Fatalf("active expiry = %s", shifted.ValidUntil)
	}
	unchanged, _ := store.PurchaseByID(ctx, inactive.ID)
	if !unchanged.ValidUntil.Equal(now) {
		t.Fatalf("inactive purchase was resurrected: %s", unchanged.ValidUntil)
	}
	var notifications, linked int
	_ = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE event_key LIKE ?`,
		"compensation:"+approved.Operation.ID+":%").Scan(&notifications)
	_ = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM node_compensation_events WHERE id=? AND provider_operation_id=?`,
		event.ID, approved.Operation.ID).Scan(&linked)
	if notifications != 1 || linked != 1 {
		t.Fatalf("notifications=%d linked=%d", notifications, linked)
	}
	var notificationKind, payloadJSON string
	if err := store.DB().QueryRowContext(ctx, `SELECT kind,payload_json FROM user_notification_events WHERE event_key LIKE ?`,
		"compensation:"+approved.Operation.ID+":%").Scan(&notificationKind, &payloadJSON); err != nil {
		t.Fatal(err)
	}
	var payload jobpayload.UserNotification
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if notificationKind != jobpayload.UserEventNodeCompensation || payload.Facts["node"] != "Review node" ||
		payload.Facts["affectedSquads"] != squad.Name || payload.Facts["downtimeSeconds"] != "120" ||
		payload.Facts["addedSeconds"] != "1020" {
		t.Fatalf("compensation notification = %s %+v", notificationKind, payload.Facts)
	}
	replayed, err := store.ApproveCompensationEvent(ctx, input, "approve-fingerprint", now.Add(4*time.Minute))
	if err != nil || replayed.Operation == nil || replayed.Operation.ID != approved.Operation.ID {
		t.Fatalf("approval replay = %+v, %v", replayed, err)
	}
	stale := input
	stale.IdempotencyKey = "stale-key"
	if _, err := store.ApproveCompensationEvent(ctx, stale, "stale-fingerprint", now); !errors.Is(err, compensation.ErrConflict) {
		t.Fatalf("stale approval error = %v", err)
	}
}

func TestCompensationDismissalPersistsAndReplays(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	now := time.Date(2026, 8, 23, 6, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "compensation-dismiss", 100, 30)
	squad := saveTestSquad(t, store, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", 10, true)
	actor := createTestUser(t, store, 49_100)
	_, _ = createAdminWorkflowPurchase(t, store, 49_101, combo, now, squad.RemnaSquadUUID)
	event := recoveredCompensationEvent(t, store, now, squad.RemnaSquadUUID, squad.Name)
	input := compensation.ReviewInput{EventID: event.ID, ActorUserID: actor.ID, IdempotencyKey: "dismiss-key",
		Reason: "not customer affecting", Revision: event.Revision}
	dismissed, err := store.DismissCompensationEvent(context.Background(), input, "dismiss-fingerprint", now.Add(3*time.Minute))
	if err != nil || dismissed.Status != "dismissed" {
		t.Fatalf("dismiss = %+v, %v", dismissed, err)
	}
	replayed, err := store.DismissCompensationEvent(context.Background(), input, "dismiss-fingerprint", now.Add(4*time.Minute))
	if err != nil || replayed.Status != "dismissed" {
		t.Fatalf("dismiss replay = %+v, %v", replayed, err)
	}
}

func recoveredCompensationEvent(t *testing.T, store *Store, now time.Time, squadUUID, squadName string) compensation.Event {
	t.Helper()
	threshold, multiplier := 1, 10_000
	config := compensation.Config{Enabled: true, ThresholdMinutes: &threshold, MultiplierBPS: &multiplier}
	node := compensation.Node{UUID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Name: "Review node",
		AffectedSquads: []compensation.Squad{{UUID: squadUUID, Name: squadName}}}
	ctx := context.Background()
	if err := store.RecordCompensationObservation(ctx, config, []compensation.Node{node}, now); err != nil {
		t.Fatal(err)
	}
	node.Connected = true
	if err := store.RecordCompensationObservation(ctx, config, []compensation.Node{node}, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListCompensationEvents(ctx, "pending_review", "", 25)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("pending event = %+v, %v", page, err)
	}
	return page.Items[0]
}

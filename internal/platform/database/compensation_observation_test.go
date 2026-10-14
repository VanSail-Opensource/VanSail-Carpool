package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
)

func TestCompensationObservationPersistsMissingDisabledAndRepeatedNodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "compensation-observe", 100, 30)
	squad := saveTestSquad(t, store, "88888888-8888-4888-8888-888888888888", 10, true)
	_, _ = createAdminWorkflowPurchase(t, store, 48_001, combo, now, squad.RemnaSquadUUID)
	threshold, multiplier := 1, 10_000
	config := compensation.Config{Enabled: true, ThresholdMinutes: &threshold, MultiplierBPS: &multiplier}
	node := compensation.Node{UUID: "11111111-1111-4111-8111-111111111111", Name: "Alpha",
		AffectedSquads: []compensation.Squad{{UUID: squad.RemnaSquadUUID, Name: squad.Name}}}

	if err := store.RecordCompensationObservation(ctx, config, []compensation.Node{node}, now); err != nil {
		t.Fatalf("start observation: %v", err)
	}
	restarted := NewStore(store.DB())
	if err := restarted.RecordCompensationObservation(ctx, config, nil, now.Add(time.Minute)); err != nil {
		t.Fatalf("missing node sample: %v", err)
	}
	page, err := restarted.ListCompensationEvents(ctx, "", "", 25)
	if err != nil || len(page.Items) != 1 || page.Items[0].Status != "observing" || page.Items[0].FrozenRecipientCount != 1 {
		t.Fatalf("persisted observing event = %+v, %v", page.Items, err)
	}
	changedThreshold, changedMultiplier := 90, 25_000
	changed := compensation.Config{Enabled: false, ThresholdMinutes: &changedThreshold, MultiplierBPS: &changedMultiplier}
	recovered := node
	recovered.Connected = true
	if err := restarted.RecordCompensationObservation(ctx, changed, []compensation.Node{recovered}, now.Add(time.Minute)); err != nil {
		t.Fatalf("recover observation: %v", err)
	}
	page, _ = restarted.ListCompensationEvents(ctx, "", "", 25)
	if page.Items[0].Status != "ineligible" || page.Items[0].IneligibleReason == nil || *page.Items[0].IneligibleReason != "below_threshold" ||
		page.Items[0].ThresholdMinutes != threshold || page.Items[0].MultiplierBPS != multiplier {
		t.Fatalf("recovered event = %+v", page.Items[0])
	}
	if err := restarted.RecordCompensationObservation(ctx, changed, []compensation.Node{node}, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	page, _ = restarted.ListCompensationEvents(ctx, "", "", 25)
	if len(page.Items) != 1 {
		t.Fatalf("disabled configuration created %d events", len(page.Items))
	}
	if err := restarted.RecordCompensationObservation(ctx, config, []compensation.Node{node}, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	disabled := node
	disabled.Disabled = true
	if err := restarted.RecordCompensationObservation(ctx, config, []compensation.Node{disabled}, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	page, _ = restarted.ListCompensationEvents(ctx, "", "", 25)
	if len(page.Items) != 2 || page.Items[0].IneligibleReason == nil || *page.Items[0].IneligibleReason != "node_disabled" {
		t.Fatalf("disabled event history = %+v", page.Items)
	}
}

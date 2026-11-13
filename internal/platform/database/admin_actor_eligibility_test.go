package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCatalogEmbyAndPricingMutationsRecheckCurrentActorEligibility(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 89103)
	combo := saveTestCombo(t, store, "Before", 100, 30)
	initial, err := store.EmbyConnection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := store.PricingPolicy(ctx, "combo", combo.ID)
	if err != nil {
		t.Fatal(err)
	}
	pair := EmbyConnectionRecord{BaseURL: Setting{Key: EmbyURLSetting, Value: "sealed-url", Encrypted: true},
		Token: Setting{Key: EmbyTokenSetting, Value: "sealed-token", Encrypted: true}}
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0},
		{"purging", "admin", "purging", 0},
		{"blocked", "admin", "active", 1},
	} {
		t.Run(state.name, func(t *testing.T) {
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, state.role, state.status, state.blocked, actor.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.SaveAdminCombo(ctx, actor.ID, ComboInput{ID: combo.ID, Name: "After", PriceTXBMinor: 200, ValidityDays: 30, TrafficLimitBytes: 1024, ResetStrategy: "DAY", Active: true}); !errors.Is(err, ErrConflict) {
				t.Fatal("ineligible actor saved combo", err)
			}
			if err := store.DeleteAdminCombo(ctx, actor.ID, combo.ID); !errors.Is(err, ErrConflict) {
				t.Fatal("ineligible actor hid combo", err)
			}
			if _, err := store.SaveAdminSquadProduct(ctx, actor.ID, SquadProductInput{GroupID: "squad", UpstreamPresent: true, Visible: true}); !errors.Is(err, ErrConflict) {
				t.Fatal("ineligible actor changed squad", err)
			}
			if err := store.SaveEmbyConnection(ctx, initial.Version(), pair, actor.ID); !errors.Is(err, ErrConflict) {
				t.Fatal("ineligible actor changed Emby connection", err)
			}
			if _, err := store.SavePricingPolicy(ctx, actor.ID, "pricing policy change", policy, time.Now()); !errors.Is(err, ErrConflict) {
				t.Fatal("ineligible actor changed pricing policy", err)
			}
		})
	}
	stored, err := store.ComboByID(ctx, combo.ID, false)
	if err != nil || stored.Name != combo.Name || !stored.Active {
		t.Fatal("rejected actors mutated catalog", stored, err)
	}
	if _, err := store.SquadProductByGroupID(ctx, "squad"); !errors.Is(err, ErrNotFound) {
		t.Fatal("rejected actor created override", err)
	}
	current, err := store.EmbyConnection(ctx)
	if err != nil || current.Version() != initial.Version() {
		t.Fatal("rejected actor changed connection", err)
	}
	currentPolicy, err := store.PricingPolicy(ctx, "combo", combo.ID)
	if err != nil || currentPolicy.Version != policy.Version || currentPolicy.LastApprovedVersion != policy.LastApprovedVersion {
		t.Fatal("rejected actor changed pricing policy", err)
	}
	events, err := store.ListAuditEvents(ctx, 100)
	if err != nil || len(events) != 0 {
		t.Fatal("rejected writes left success audits", events, err)
	}
}

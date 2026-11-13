package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

const manualGrantSquadID = "77777777-7777-4777-8777-777777777777"

type adminGrantRepositoryStub struct {
	userWorkflowRepositoryStub
	combo      model.Combo
	comboErr   error
	activeOnly bool
	created    *database.AdminEntitlementCreateInput
	replay     database.AdminEntitlementGrantReplay
	replayErr  error
}

func (r *adminGrantRepositoryStub) AdminEntitlementGrantByKey(context.Context, string, string, string) (database.AdminEntitlementGrantReplay, bool, error) {
	return r.replay, r.replay.Purchase.ID != "", r.replayErr
}

func (r *adminGrantRepositoryStub) ComboByID(_ context.Context, _ string, activeOnly bool) (model.Combo, error) {
	r.activeOnly = activeOnly
	return r.combo, r.comboErr
}

func (r *adminGrantRepositoryStub) CreateAdminEntitlement(_ context.Context, input database.AdminEntitlementCreateInput,
	_ time.Time) (model.Purchase, error) {
	r.created = &input
	return model.Purchase{ID: "manual-purchase", UserID: input.UserID}, nil
}

func TestCreateEntitlementValidatesAndSnapshotsManualGrant(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	repository := &adminGrantRepositoryStub{combo: model.Combo{ID: "combo-1", Active: true,
		IncludedSquads: []model.SquadProduct{{GroupID: manualGrantSquadID}}}}
	workflows := NewUserWorkflows(repository, &adminSquadImporter{squads: []UpstreamSquad{{UUID: manualGrantSquadID}}})
	workflows.now = func() time.Time { return now }

	purchase, err := workflows.CreateEntitlement(t.Context(), "admin-1", "user-1", "grant-key", AdminEntitlementCreate{
		Reason: "  manual fixed-term grant  ", ComboID: " combo-1 ", ValidFrom: now.Add(-24 * time.Hour),
		ValidUntil: now.Add(30 * 24 * time.Hour), TrafficLimitBytes: 4_096, ResetStrategy: "DAY",
		SquadUUIDs: []string{manualGrantSquadID}})
	if err != nil || purchase.ID != "manual-purchase" {
		t.Fatalf("CreateEntitlement() = %+v, %v", purchase, err)
	}
	if !repository.activeOnly || repository.created == nil {
		t.Fatal("manual grant did not require an active combo or reach the store")
	}
	input := repository.created
	if input.ActorUserID != "admin-1" || input.UserID != "user-1" || input.IdempotencyKey != "grant-key" ||
		input.Reason != "manual fixed-term grant" || input.ComboID != "combo-1" || len(input.RequestFingerprint) != 64 ||
		input.TrafficLimitBytes != 4_096 || input.ResetStrategy != "DAY" ||
		!input.ValidFrom.Equal(now.Add(-24*time.Hour)) || !input.ValidUntil.Equal(now.Add(30*24*time.Hour)) {
		t.Fatalf("grant command = %+v", input)
	}
	if len(input.SquadUUIDs) != 1 || input.SquadUUIDs[0] != manualGrantSquadID {
		t.Fatalf("grant squads = %v", input.SquadUUIDs)
	}
}

func TestCreateEntitlementInheritsComboSquadsAndRejectsInvalidInput(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	repository := &adminGrantRepositoryStub{combo: model.Combo{ID: "combo-1", Active: true,
		IncludedSquads: []model.SquadProduct{{GroupID: manualGrantSquadID}}}}
	workflows := NewUserWorkflows(repository, &adminSquadImporter{squads: []UpstreamSquad{{UUID: manualGrantSquadID}}})
	workflows.now = func() time.Time { return now }
	valid := AdminEntitlementCreate{Reason: "manual grant", ComboID: "combo-1", ValidFrom: now,
		ValidUntil: now.Add(30 * 24 * time.Hour), TrafficLimitBytes: 4_096, ResetStrategy: "DAY"}
	if _, err := workflows.CreateEntitlement(t.Context(), "admin-1", "user-1", "inherit-squads", valid); err != nil {
		t.Fatalf("CreateEntitlement(inherit): %v", err)
	}
	if repository.created == nil || len(repository.created.SquadUUIDs) != 1 || repository.created.SquadUUIDs[0] != manualGrantSquadID {
		t.Fatalf("inherited squads = %+v", repository.created)
	}

	for name, mutate := range map[string]func(*AdminEntitlementCreate){
		"short reason":  func(value *AdminEntitlementCreate) { value.Reason = "no" },
		"zero traffic":  func(value *AdminEntitlementCreate) { value.TrafficLimitBytes = 0 },
		"reversed term": func(value *AdminEntitlementCreate) { value.ValidUntil = value.ValidFrom },
		"bad reset":     func(value *AdminEntitlementCreate) { value.ResetStrategy = "YEAR" },
		"missing combo": func(value *AdminEntitlementCreate) { value.ComboID = " " },
		"unknown squad": func(value *AdminEntitlementCreate) {
			value.SquadUUIDs = []string{manualGrantSquadID[1:] + manualGrantSquadID[:1]}
		},
	} {
		value := valid
		mutate(&value)
		if _, err := workflows.CreateEntitlement(t.Context(), "admin-1", "user-1", "invalid-"+name, value); err == nil {
			t.Fatalf("CreateEntitlement(%s) accepted invalid input", name)
		}
	}
	repository.comboErr = database.ErrNotFound
	if _, err := workflows.CreateEntitlement(t.Context(), "admin-1", "user-1", "missing-combo-2", valid); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("missing combo = %v", err)
	}
}

func TestCreateEntitlementRequiresAnEntitlementCreatingStore(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	workflows := NewUserWorkflows(&userWorkflowRepositoryStub{}, nil)
	workflows.now = func() time.Time { return now }
	_, err := workflows.CreateEntitlement(t.Context(), "admin-1", "user-1", "no-store", AdminEntitlementCreate{
		Reason: "manual grant", ComboID: "combo-1", ValidFrom: now, ValidUntil: now.Add(24 * time.Hour),
		TrafficLimitBytes: 1_024, ResetStrategy: "DAY"})
	if err == nil {
		t.Fatal("grant succeeded without a durable entitlement creator")
	}
}

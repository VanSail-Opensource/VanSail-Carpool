package admin

import (
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestManualGrantReplayUsesAcceptedSquadsBeforeCatalogOrProviderPreflight(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	repository := &adminGrantRepositoryStub{combo: model.Combo{ID: "combo", IncludedSquads: []model.SquadProduct{{GroupID: manualGrantSquadID}}}}
	workflow := NewUserWorkflows(repository, &adminSquadImporter{squads: []UpstreamSquad{{UUID: manualGrantSquadID}}})
	request := AdminEntitlementCreate{Reason: "manual grant", ComboID: "combo", ValidFrom: now,
		ValidUntil: now.Add(24 * time.Hour), TrafficLimitBytes: 4096, ResetStrategy: "DAY"}
	if _, err := workflow.CreateEntitlement(t.Context(), "actor", "owner", "accepted", request); err != nil {
		t.Fatal(err)
	}
	accepted := *repository.created
	repository.replay = database.AdminEntitlementGrantReplay{RequestFingerprint: accepted.RequestFingerprint,
		Purchase: model.Purchase{ID: "original-grant", UserID: "owner", ComboID: "combo", SquadUUIDs: accepted.SquadUUIDs}}
	repository.created, repository.activeOnly, repository.comboErr = nil, false, database.ErrNotFound
	workflow.importer = nil
	replayed, err := workflow.CreateEntitlement(t.Context(), "actor", "owner", "accepted", request)
	if err != nil || replayed.ID != "original-grant" || repository.created != nil || repository.activeOnly {
		t.Fatal("replay consulted mutable catalog/provider or created work", replayed, err)
	}
	for name, mutate := range map[string]func(*AdminEntitlementCreate){
		"reason":  func(value *AdminEntitlementCreate) { value.Reason = "different reason" },
		"combo":   func(value *AdminEntitlementCreate) { value.ComboID = "changed-combo" },
		"term":    func(value *AdminEntitlementCreate) { value.ValidUntil = value.ValidUntil.Add(time.Hour) },
		"traffic": func(value *AdminEntitlementCreate) { value.TrafficLimitBytes++ },
		"reset":   func(value *AdminEntitlementCreate) { value.ResetStrategy = "WEEK" },
		"squad": func(value *AdminEntitlementCreate) {
			value.SquadUUIDs = []string{"88888888-8888-4888-8888-888888888888"}
		},
	} {
		changed := request
		mutate(&changed)
		if _, err = workflow.CreateEntitlement(t.Context(), "actor", "owner", "accepted", changed); !errors.Is(err, database.ErrConflict) {
			t.Fatal("changed command reused accepted grant", name, err)
		}
	}
	request.SquadUUIDs = accepted.SquadUUIDs
	if _, err = workflow.CreateEntitlement(t.Context(), "actor", "owner", "accepted", request); err != nil {
		t.Fatal("legacy effective-squad fingerprint changed", err)
	}
	repository.replayErr = errors.New("storage temporarily unavailable")
	if _, err = workflow.CreateEntitlement(t.Context(), "actor", "owner", "accepted", request); !errors.Is(err, repository.replayErr) {
		t.Fatal("replay storage cause was lost", err)
	}
}

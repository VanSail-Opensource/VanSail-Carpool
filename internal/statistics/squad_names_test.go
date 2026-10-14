package statistics

import (
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestApplySquadNames(t *testing.T) {
	statistics := model.DatabaseStatistics{
		SquadByCombo: []model.NormalizedDistribution{{
			ID: "combo-1", Label: "Core", Segments: []model.NamedShare{{ID: "squad-1", Label: "squad-1", Value: 100}},
		}},
		ComboBySquad: []model.NormalizedDistribution{{
			ID: "squad-1", Label: "squad-1", Segments: []model.NamedShare{{ID: "combo-1", Label: "Core", Value: 100}},
		}},
	}

	applySquadNames(&statistics, map[string]string{"squad-1": "Singapore"})

	if statistics.SquadByCombo[0].Segments[0].Label != "Singapore" {
		t.Fatalf("combo segment label = %q, want squad name", statistics.SquadByCombo[0].Segments[0].Label)
	}
	if statistics.ComboBySquad[0].Label != "Singapore" {
		t.Fatalf("squad group label = %q, want squad name", statistics.ComboBySquad[0].Label)
	}
}

func TestCachedSquadNamesReturnsCopy(t *testing.T) {
	service := &Service{snapshot: model.ProductStatisticsSnapshot{
		Database: model.DatabaseStatistics{ComboBySquad: []model.NormalizedDistribution{{
			ID: "squad-1", Label: "Singapore",
		}}},
	}}

	names := service.cachedSquadNames()
	names["squad-1"] = "Changed"

	if service.snapshot.Database.ComboBySquad[0].Label != "Singapore" {
		t.Fatal("cached squad names exposed shared state")
	}
}

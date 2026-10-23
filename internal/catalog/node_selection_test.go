package catalog

import (
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestSelectedSquadUUIDs(t *testing.T) {
	t.Parallel()
	catalog := model.Catalog{
		Combos: []model.Combo{{ID: "core", IncludedSquads: []model.SquadProduct{
			{GroupID: "core-squad"}, {GroupID: "shared-squad"},
		}}},
		Addons: []model.SquadProduct{{ID: "addon-id", GroupID: "addon-squad"}},
	}
	tests := []struct {
		name     string
		comboID  string
		addonIDs []string
		want     []string
	}{
		{"deduplicates known selections", "core", []string{" addon-id ", "addon-squad", "unknown"}, []string{"addon-squad", "core-squad", "shared-squad"}},
		{"ignores unknown selections", "unknown", []string{"unknown"}, []string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selectedSquadUUIDs(catalog, test.comboID, test.addonIDs); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("selectedSquadUUIDs() = %v, want %v", got, test.want)
			}
		})
	}
}

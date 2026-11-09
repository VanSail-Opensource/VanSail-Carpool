package catalog

import (
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestProjectCatalogNodesFiltersDeduplicatesAndSorts(t *testing.T) {
	t.Parallel()
	nodes := projectCatalogNodes([]RemoteNode{
		{UUID: "zulu", Name: "Zulu", CountryCode: "US"},
		{UUID: "disabled", Name: "Disabled", CountryCode: "DE", Disabled: true},
		{UUID: "berlin", Name: "Berlin", CountryCode: "DE", ProviderName: " Provider "},
		{UUID: "berlin", Name: "Duplicate", CountryCode: "AU"},
	})
	provider := "Provider"
	want := []model.CatalogNode{
		{UUID: "berlin", Name: "Berlin", CountryCode: "DE", ConsumptionMultiplier: 1, ProviderName: &provider},
		{UUID: "zulu", Name: "Zulu", CountryCode: "US", ConsumptionMultiplier: 1},
	}
	if !reflect.DeepEqual(nodes, want) {
		t.Fatalf("projectCatalogNodes() = %#v, want %#v", nodes, want)
	}
}

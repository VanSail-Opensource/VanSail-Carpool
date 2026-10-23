package catalog

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestCatalogGroupsAccessibleNodesPerUniqueSquad(t *testing.T) {
	t.Parallel()
	remote := &catalogNodeRemote{
		catalogProxy: &catalogProxy{},
		squads: []RemoteSquad{
			{UUID: "core-squad", Name: "Core"},
			{UUID: "shared-squad", Name: "Shared"},
			{UUID: "addon-squad", Name: "Extra"},
		},
		nodes: []RemoteNode{
			{UUID: "node-a", Name: "Alpha", CountryCode: "DE"},
			{UUID: "node-b", Name: "Beta", CountryCode: "JP"},
		},
		accessible: map[string][]string{
			"core-squad": {"node-b"}, "shared-squad": {"node-a"}, "addon-squad": {"node-a", "node-a"},
		},
	}
	catalog, err := NewService(nodeCatalogRepository(), remote, time.Minute).Catalog(context.Background())
	if err != nil {
		t.Fatalf("Catalog(): %v", err)
	}
	if got := catalog.Combos[0].IncludedSquads[0].AccessibleNodes[0].UUID; got != "node-b" {
		t.Fatalf("core accessible node = %q, want node-b", got)
	}
	if got := catalog.Addons[0].AccessibleNodes; len(got) != 1 || got[0].UUID != "node-a" {
		t.Fatalf("add-on accessible nodes = %#v, want deduplicated node-a", got)
	}
	if !reflect.DeepEqual(remote.requestedSquads, []string{"addon-squad", "core-squad", "shared-squad"}) {
		t.Fatalf("accessible lookups = %v", remote.requestedSquads)
	}
}

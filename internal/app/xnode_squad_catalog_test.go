package app

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type squadCatalogRepository struct{ xnode.Repository }

func (squadCatalogRepository) XNodeGroups(context.Context) ([]xnode.Group, error) {
	return []xnode.Group{{ID: "enabled", Name: "Enabled", Enabled: true}, {ID: "disabled", Name: "Disabled"}}, nil
}
func TestNativeCatalogOmitsDisabledGroupsWithoutChangingOwnedNames(t *testing.T) {
	provider := proxyProvider{native: xnode.NewService(squadCatalogRepository{}, nil, nil, nil, nil)}
	groups, err := provider.ListCatalogSquads(context.Background())
	if err != nil || len(groups) != 1 || groups[0].UUID != "enabled" {
		t.Fatal(groups, err)
	}
	names, err := provider.MemberSquadNames(context.Background())
	if err != nil || names["disabled"] != "Disabled" {
		t.Fatal("owned group display names disappeared", names, err)
	}
}

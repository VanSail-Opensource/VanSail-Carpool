package catalog

import (
	"context"
	"sort"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// RemoteSquad is the live identity portion of a Proxy internal squad.
type RemoteSquad struct {
	UUID string
	Name string
}

type remoteSquadLister interface {
	ListCatalogSquads(context.Context) ([]RemoteSquad, error)
}

// Catalog overlays local merchandising on the live Proxy squad list. A
// combo with a stale included UUID is omitted until the upstream identity is
// restored, preventing checkout against a locally cached catalog.
func (s *Service) Catalog(ctx context.Context) (model.Catalog, error) {
	combos, err := s.repository.ListCombos(ctx, true)
	if err != nil {
		return model.Catalog{}, err
	}
	// Load hidden overrides too so combo-included squads retain activation and
	// profile metadata; hydrateLiveCatalog still filters hidden standalone add-ons.
	addons, err := s.repository.ListSquadProducts(ctx, false)
	if err != nil {
		return model.Catalog{}, err
	}
	return s.hydrateLiveCatalog(ctx, combos, addons)
}

func (s *Service) hydrateLiveCatalog(ctx context.Context, combos []model.Combo, overrides []model.SquadProduct) (model.Catalog, error) {
	var provider any = s.provider
	if source, ok := provider.(remoteTopologyLister); ok {
		snapshot, err := source.ListCatalogTopology(ctx)
		if err != nil {
			return model.Catalog{}, err
		}
		provider = topologySnapshot{snapshot}
	}
	lister, ok := provider.(remoteSquadLister)
	if !ok {
		if nodeLister, available := provider.(remoteNodeLister); available {
			return hydrateStoredCatalogNodes(ctx, nodeLister, combos, overrides)
		}
		return catalogWithEmptyNodes(combos, overrides), nil
	}
	remote, err := lister.ListCatalogSquads(ctx)
	if err != nil {
		return model.Catalog{}, err
	}
	live := make(map[string]string, len(remote))
	for _, squad := range remote {
		live[strings.TrimSpace(squad.UUID)] = strings.TrimSpace(squad.Name)
	}
	uniqueSquads := make(map[string]struct{})
	for _, override := range overrides {
		if _, present := live[override.GroupID]; present {
			uniqueSquads[override.GroupID] = struct{}{}
		}
	}
	for _, combo := range combos {
		for _, included := range combo.IncludedSquads {
			if _, present := live[included.GroupID]; present {
				uniqueSquads[included.GroupID] = struct{}{}
			}
		}
	}
	squadUUIDs := make([]string, 0, len(uniqueSquads))
	for squadUUID := range uniqueSquads {
		squadUUIDs = append(squadUUIDs, squadUUID)
	}
	sort.Strings(squadUUIDs)
	catalogNodes := []model.CatalogNode{}
	nodesBySquad := make(map[string][]model.CatalogNode)
	if nodeLister, ok := provider.(remoteNodeLister); ok {
		catalogNodes, nodesBySquad, err = hydrateSquadNodes(ctx, nodeLister, squadUUIDs)
		if err != nil {
			return model.Catalog{}, err
		}
	}
	overrideByUUID := make(map[string]model.SquadProduct, len(overrides))
	addons := make([]model.SquadProduct, 0, len(overrides))
	for _, override := range overrides {
		name, present := live[override.GroupID]
		if !present {
			continue
		}
		override.Name = name
		override.UpstreamPresent = true
		override.AccessibleNodes = nodesBySquad[override.GroupID]
		if override.AccessibleNodes == nil {
			override.AccessibleNodes = []model.CatalogNode{}
		}
		overrideByUUID[override.GroupID] = override
		if override.Visible && override.BillingMode != "payg" {
			addons = append(addons, override)
		}
	}
	liveCombos := make([]model.Combo, 0, len(combos))
	for _, combo := range combos {
		included := make([]model.SquadProduct, 0, len(combo.IncludedSquads))
		valid := true
		for _, placeholder := range combo.IncludedSquads {
			name, present := live[placeholder.GroupID]
			if !present {
				valid = false
				break
			}
			product, hasOverride := overrideByUUID[placeholder.GroupID]
			if product.BillingMode == "payg" {
				continue
			}
			if !hasOverride {
				product = model.SquadProduct{ID: placeholder.GroupID, GroupID: placeholder.GroupID, Visible: true,
					AccessibleNodes: []model.CatalogNode{}}
			}
			product.Name = name
			product.UpstreamPresent = true
			product.AccessibleNodes = nodesBySquad[placeholder.GroupID]
			if product.AccessibleNodes == nil {
				product.AccessibleNodes = []model.CatalogNode{}
			}
			included = append(included, product)
		}
		if valid {
			combo.IncludedSquads = included
			liveCombos = append(liveCombos, combo)
		}
	}
	return model.Catalog{Combos: liveCombos, Addons: addons, Nodes: catalogNodes}, nil
}

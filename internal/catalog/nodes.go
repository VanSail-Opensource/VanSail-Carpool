package catalog

import (
	"context"
	"sort"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// RemoteNode is the non-sensitive node data needed to preview a purchase.
type RemoteNode struct {
	UUID                  string
	Name                  string
	CountryCode           string
	ConsumptionMultiplier float64
	ActiveInboundUUIDs    []string
	Disabled              bool
	ProviderName          string
	ProviderFaviconURL    *string
}

type remoteNodeLister interface {
	ListCatalogNodes(context.Context) ([]RemoteNode, error)
	AccessibleCatalogNodeUUIDs(context.Context, string) ([]string, error)
}

func projectCatalogNodes(nodes []RemoteNode) []model.CatalogNode {
	unique := make(map[string]model.CatalogNode, len(nodes))
	for _, node := range nodes {
		node.UUID = strings.TrimSpace(node.UUID)
		if node.Disabled || node.UUID == "" {
			continue
		}
		if _, exists := unique[node.UUID]; exists {
			continue
		}
		var providerName *string
		if trimmed := strings.TrimSpace(node.ProviderName); trimmed != "" {
			providerName = &trimmed
		}
		unique[node.UUID] = model.CatalogNode{
			UUID:                  node.UUID,
			Name:                  node.Name,
			CountryCode:           node.CountryCode,
			ConsumptionMultiplier: node.ConsumptionMultiplier,
			ProviderName:          providerName,
			ActiveInboundUUIDs:    append([]string(nil), node.ActiveInboundUUIDs...),
			ProviderFaviconURL:    node.ProviderFaviconURL,
		}
	}
	result := make([]model.CatalogNode, 0, len(unique))
	for _, node := range unique {
		result = append(result, node)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].CountryCode == result[right].CountryCode {
			if result[left].Name == result[right].Name {
				return result[left].UUID < result[right].UUID
			}
			return result[left].Name < result[right].Name
		}
		return result[left].CountryCode < result[right].CountryCode
	})
	return result
}

func hydrateSquadNodes(ctx context.Context, provider remoteNodeLister, squadUUIDs []string) ([]model.CatalogNode, map[string][]model.CatalogNode, error) {
	remoteNodes, err := provider.ListCatalogNodes(ctx)
	if err != nil {
		return nil, nil, err
	}
	allNodes := projectCatalogNodes(remoteNodes)
	byUUID := make(map[string]model.CatalogNode, len(allNodes))
	for _, node := range allNodes {
		byUUID[node.UUID] = node
	}
	bySquad := make(map[string][]model.CatalogNode, len(squadUUIDs))
	for _, squadUUID := range squadUUIDs {
		accessible, lookupErr := provider.AccessibleCatalogNodeUUIDs(ctx, squadUUID)
		if lookupErr != nil {
			return nil, nil, lookupErr
		}
		seen := make(map[string]struct{}, len(accessible))
		for _, nodeUUID := range accessible {
			node, exists := byUUID[strings.TrimSpace(nodeUUID)]
			if !exists {
				continue
			}
			if _, duplicate := seen[node.UUID]; duplicate {
				continue
			}
			seen[node.UUID] = struct{}{}
			bySquad[squadUUID] = append(bySquad[squadUUID], node)
		}
		sortCatalogNodes(bySquad[squadUUID])
	}
	return allNodes, bySquad, nil
}

func hydrateStoredCatalogNodes(ctx context.Context, provider remoteNodeLister, combos []model.Combo, addons []model.SquadProduct) (model.Catalog, error) {
	unique := make(map[string]struct{})
	for _, combo := range combos {
		for _, squad := range combo.IncludedSquads {
			unique[squad.GroupID] = struct{}{}
		}
	}
	for _, squad := range addons {
		unique[squad.GroupID] = struct{}{}
	}
	squadUUIDs := make([]string, 0, len(unique))
	for squadUUID := range unique {
		squadUUIDs = append(squadUUIDs, squadUUID)
	}
	sort.Strings(squadUUIDs)
	nodes, bySquad, err := hydrateSquadNodes(ctx, provider, squadUUIDs)
	if err != nil {
		return model.Catalog{}, err
	}
	for comboIndex := range combos {
		for squadIndex := range combos[comboIndex].IncludedSquads {
			squad := &combos[comboIndex].IncludedSquads[squadIndex]
			squad.AccessibleNodes = nonNilCatalogNodes(bySquad[squad.GroupID])
		}
	}
	for index := range addons {
		addons[index].AccessibleNodes = nonNilCatalogNodes(bySquad[addons[index].GroupID])
	}
	return model.Catalog{Combos: combos, Addons: addons, Nodes: nodes}, nil
}

func nonNilCatalogNodes(nodes []model.CatalogNode) []model.CatalogNode {
	if nodes == nil {
		return []model.CatalogNode{}
	}
	return nodes
}

func catalogWithEmptyNodes(combos []model.Combo, addons []model.SquadProduct) model.Catalog {
	for comboIndex := range combos {
		for squadIndex := range combos[comboIndex].IncludedSquads {
			combos[comboIndex].IncludedSquads[squadIndex].AccessibleNodes = []model.CatalogNode{}
		}
	}
	for index := range addons {
		addons[index].AccessibleNodes = []model.CatalogNode{}
	}
	return model.Catalog{Combos: combos, Addons: addons, Nodes: []model.CatalogNode{}}
}

func quoteAccessibleNodes(catalog model.Catalog, comboID string, addonIDs []string) []model.ProxyNode {
	squadUUIDs := selectedSquadUUIDs(catalog, comboID, addonIDs)
	bySquad := make(map[string][]model.CatalogNode, len(catalog.Addons)+len(catalog.Combos))
	for _, combo := range catalog.Combos {
		for _, squad := range combo.IncludedSquads {
			bySquad[squad.GroupID] = squad.AccessibleNodes
		}
	}
	for _, squad := range catalog.Addons {
		bySquad[squad.GroupID] = squad.AccessibleNodes
	}
	accessible := make(map[string]model.CatalogNode)
	for _, squadUUID := range squadUUIDs {
		for _, node := range bySquad[squadUUID] {
			accessible[node.UUID] = node
		}
	}
	result := make([]model.ProxyNode, 0, len(accessible))
	for _, node := range accessible {
		providerName := ""
		if node.ProviderName != nil {
			providerName = *node.ProviderName
		}
		result = append(result, model.ProxyNode{UUID: node.UUID, Name: node.Name, CountryCode: node.CountryCode,
			ConsumptionMultiplier: node.ConsumptionMultiplier, ActiveInboundUUIDs: node.ActiveInboundUUIDs, Accessible: true,
			ProviderName: providerName, ProviderFaviconURL: node.ProviderFaviconURL})
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].CountryCode == result[right].CountryCode {
			return result[left].Name < result[right].Name
		}
		return result[left].CountryCode < result[right].CountryCode
	})
	return result
}

func sortCatalogNodes(nodes []model.CatalogNode) {
	sort.Slice(nodes, func(left, right int) bool {
		if nodes[left].CountryCode == nodes[right].CountryCode {
			if nodes[left].Name == nodes[right].Name {
				return nodes[left].UUID < nodes[right].UUID
			}
			return nodes[left].Name < nodes[right].Name
		}
		return nodes[left].CountryCode < nodes[right].CountryCode
	})
}

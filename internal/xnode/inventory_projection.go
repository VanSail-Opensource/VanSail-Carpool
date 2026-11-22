package xnode

import (
	"context"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// inventoryProjection belongs to one sequential inventory/metrics read. It
// reuses topology and decoded inbound inputs, never member eligibility, balances
// or periods. A new read (including a subscription retry) starts from storage.
type inventoryProjection struct {
	service         *Service
	groups          []Group
	groupsLoaded    bool
	bindings        map[[2]string][]string
	endpoints       []Endpoint
	endpointsLoaded bool
	nodes           map[string]Node
	inbounds        map[string]inventoryInbounds
	credentials     map[[2]string]Credentials
}

type inventoryInbounds struct {
	node     Node
	settings NodeSettings
	byTag    map[string]map[string]any
}

func newInventoryProjection(service *Service) *inventoryProjection {
	return &inventoryProjection{service: service, nodes: map[string]Node{}, inbounds: map[string]inventoryInbounds{}, credentials: map[[2]string]Credentials{}}
}

func (p *inventoryProjection) readGroups(ctx context.Context) ([]Group, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !p.groupsLoaded {
		groups, err := p.service.repo.XNodeGroups(ctx)
		if err != nil {
			return nil, err
		}
		p.groups, p.groupsLoaded = groups, true
		p.bindings = map[[2]string][]string{}
		for _, group := range groups {
			for _, binding := range group.Bindings {
				key := [2]string{binding.NodeID, binding.InboundTag}
				if !slices.Contains(p.bindings[key], group.ID) {
					p.bindings[key] = append(p.bindings[key], group.ID)
				}
			}
		}
	}
	return p.groups, nil
}

func (p *inventoryProjection) readEndpoints(ctx context.Context) ([]Endpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !p.endpointsLoaded {
		items, err := p.service.Endpoints(ctx)
		if err != nil {
			return nil, err
		}
		p.endpoints, p.endpointsLoaded = items, true
	}
	return p.endpoints, nil
}

func (p *inventoryProjection) readNode(ctx context.Context, id string) (Node, error) {
	if err := ctx.Err(); err != nil {
		return Node{}, err
	}
	if node, ok := p.nodes[id]; ok {
		return node, nil
	}
	node, err := p.service.repo.XNodeNode(ctx, id)
	if err == nil {
		p.nodes[id] = node
	}
	return node, err
}

// Callers load groups before filtering. A binding is only a topology filter;
// enabled products, owner access and endpoint ACLs still decide eligibility.
func (p *inventoryProjection) endpointGroups(endpoint Endpoint) []string {
	return p.bindings[[2]string{endpoint.NodeID, endpoint.InboundTag}]
}

func (p *inventoryProjection) proxy(ctx context.Context, identity Identity, endpoint Endpoint) (subscription.Proxy, error) {
	if err := ctx.Err(); err != nil {
		return subscription.Proxy{}, err
	}
	key := [2]string{identityScope(identity), identity.CredentialsCipher}
	credentials, ok := p.credentials[key]
	if !ok {
		var err error
		credentials, err = loadCredentials(p.service.vault, identity)
		if err != nil {
			return subscription.Proxy{}, err
		}
		p.credentials[key] = credentials
	}
	if _, err := p.readGroups(ctx); err != nil {
		return subscription.Proxy{}, err
	}
	inputs, ok := p.inbounds[endpoint.NodeID]
	if !ok {
		node, settings, inbounds, err := p.service.endpointInbounds(ctx, endpoint.NodeID)
		if err != nil {
			return subscription.Proxy{}, err
		}
		inputs = inventoryInbounds{node: node, settings: settings, byTag: map[string]map[string]any{}}
		for _, inbound := range inbounds {
			tag, _ := inbound["tag"].(string)
			if _, exists := inputs.byTag[tag]; !exists {
				inputs.byTag[tag] = inbound
			}
		}
		p.inbounds[endpoint.NodeID] = inputs
	}
	inbound := inputs.byTag[endpoint.InboundTag]
	if inbound == nil {
		return subscription.Proxy{}, ErrInvalid
	}
	return projectEndpointProxy(endpoint, credentials, p.endpointGroups(endpoint), inputs.node, inputs.settings, inbound)
}

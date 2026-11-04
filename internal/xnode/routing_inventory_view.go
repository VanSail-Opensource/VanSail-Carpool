package xnode

import "context"

// Retain the latest labels for temporarily suspended endpoints. Selection is
// always by stable ID; unavailable rows are visible but cannot be newly selected.
func (s *Service) appendUnavailableRoutingEndpoints(ctx context.Context, user string, view *RoutingView) error {
	entitled, err := s.routingEntitlementEndpoints(ctx, user)
	if err != nil {
		return err
	}
	seen, nodes := map[string]bool{}, map[string]bool{}
	for _, endpoint := range view.Endpoints {
		seen[endpoint.ID] = true
	}
	for _, node := range view.Nodes {
		nodes[node.ID] = true
	}
	for _, endpoint := range entitled.byEndpoint {
		if !seen[endpoint.ID] {
			view.Endpoints = append(view.Endpoints, RoutingEndpoint{ID: endpoint.ID, NodeID: endpoint.NodeID, Name: endpoint.Name, Address: endpoint.Address, Port: endpoint.Port})
		}
		if !nodes[endpoint.NodeID] {
			node, err := s.repo.XNodeNode(ctx, endpoint.NodeID)
			if err != nil {
				if isMissing(err) {
					continue
				}
				return err
			}
			view.Nodes = append(view.Nodes, RoutingNode{ID: node.ID, Name: node.Name})
			nodes[node.ID] = true
		}
	}
	return nil
}

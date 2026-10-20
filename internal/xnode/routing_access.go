package xnode

import (
	"context"
	"fmt"
	"time"
)

type routingAccessRepository interface {
	PruneXNodeRoutingAccess(context.Context, RoutingPlan, string, int64, time.Time) error
}

// Access comes from durable entitlements and endpoint ownership, never node
// telemetry. Read failures preserve the plan; a successful empty inventory
// authoritatively removes every related choice.
func (s *Service) pruneRoutingAccess(ctx context.Context, plan RoutingPlan, identity Identity, doc RoutingDocument) (bool, error) {
	if identity.UserID == "" || len(doc.Choices) == 0 {
		return false, nil
	}
	repo, ok := s.routingRepository()
	if !ok {
		return false, ErrUnavailable
	}
	writer, ok := repo.(routingAccessRepository)
	if !ok {
		return false, nil
	}
	versions, ok := repo.(compileVersionRepository)
	if !ok {
		return false, nil
	}
	version, boundary, err := versions.XNodeCompileVersion(ctx, s.now())
	if err != nil {
		return false, err
	}
	in := routingInput{endpoints: map[string]Endpoint{}}
	if err := s.loadRoutingEndpoints(ctx, identity, &in); err != nil {
		return false, err
	}
	kept := accessibleRoutingChoices(doc.Choices, in)
	if len(kept) == len(doc.Choices) {
		return false, nil
	}
	// Keep node IDs for in-flight revocation, but erase previous operational
	// choices. Historical encrypted conversion/audit evidence remains immutable.
	nodes, err := s.routingPlanNodes(plan)
	if err != nil {
		return false, err
	}
	doc.Choices, doc.LegacyNodes, doc.Dependencies = kept, nodes, ""
	plan.Cipher, err = s.seal(fmt.Sprintf("xnode.routes.%d", plan.ProxyUserID), doc)
	if err != nil {
		return false, err
	}
	if !boundary.IsZero() && !s.now().Before(boundary) {
		return false, ErrConflict
	}
	if err := writer.PruneXNodeRoutingAccess(ctx, plan, identity.UserID, version, s.now().UTC()); err != nil {
		return false, err
	}
	return true, nil
}

func accessibleRoutingChoices(choices []RoutingChoice, in routingInput) []RoutingChoice {
	kept := make([]RoutingChoice, 0, len(choices))
	for _, choice := range choices {
		source, target := in.byEndpoint[choice.InboundEndpointID], in.byEndpoint[choice.OutboundEndpointID]
		if choice.InboundEndpointID == "" {
			source = in.endpoints[choice.InboundNodeID]
		}
		if choice.OutboundEndpointID == "" {
			target = in.endpoints[choice.OutboundNodeID]
		}
		if source.ID != "" && target.ID != "" && source.NodeID == choice.InboundNodeID && target.NodeID == choice.OutboundNodeID {
			kept = append(kept, choice)
		}
	}
	return kept
}

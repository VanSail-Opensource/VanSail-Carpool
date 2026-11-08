package xnode

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

const routingRemovalPending = "routing_node_removal_pending:"

// RoutingRemovalPending fences member saves until revocation evidence is drained.
func RoutingRemovalPending(plan RoutingPlan) bool {
	return strings.HasPrefix(plan.Reason, routingRemovalPending)
}

func (s *Service) prepareRemovalRoutes(ctx context.Context, node string) ([]RoutingPlan, error) {
	repo, ok := s.routingRepository()
	if !ok {
		return nil, ErrUnavailable
	}
	plans, err := repo.XNodeRoutingPlans(ctx)
	if err != nil {
		return nil, err
	}
	changes := []RoutingPlan{}
	for _, plan := range plans {
		nodes, err := s.routingPlanNodes(plan)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(nodes, node) {
			continue
		}
		if RoutingRemovalPending(plan) {
			if plan.Reason != routingRemovalPending+node {
				return nil, RemovalBlocked("XNODE_REMOVAL_ROUTING_PENDING")
			}
			continue
		}
		doc, err := s.routingDocument(plan)
		if err != nil {
			return nil, err
		}
		previous := doc
		previous.Choices = slices.Clone(doc.Choices)
		if plan.PreviousCipher != "" {
			old := plan
			old.Cipher = plan.PreviousCipher
			prior, err := s.routingDocument(old)
			if err != nil {
				return nil, err
			}
			previous.Choices = append(previous.Choices, prior.Choices...)
		}
		previous.LegacyNodes = nodes
		doc.Choices = slices.DeleteFunc(slices.Clone(doc.Choices), func(choice RoutingChoice) bool { return choice.InboundNodeID == node || choice.OutboundNodeID == node })
		doc.LegacyNodes = slices.DeleteFunc(slices.Clone(nodes), func(id string) bool { return id == node })
		doc.Dependencies = ""
		plan.Cipher, err = s.seal(fmt.Sprintf("xnode.routes.%d", plan.ProxyUserID), doc)
		if err != nil {
			return nil, err
		}
		plan.PreviousCipher, err = s.seal(fmt.Sprintf("xnode.routes.%d", plan.ProxyUserID), previous)
		if err != nil {
			return nil, err
		}
		changes = append(changes, plan)
	}
	return changes, nil
}

func (s *Service) drainRemovalRoutes(ctx context.Context, node string) error {
	repo, ok := s.routingRepository()
	if !ok {
		return ErrUnavailable
	}
	plans, err := repo.XNodeRoutingPlans(ctx)
	if err != nil {
		return err
	}
	for _, plan := range plans {
		if plan.Reason != routingRemovalPending+node {
			continue
		}
		nodes, err := s.routingPlanNodes(plan)
		if err != nil {
			return err
		}
		if err = s.syncRoutingNodes(ctx, nodes, plan.ProxyUserID, true, plan); err != nil {
			return &PolicyError{"XNODE_REMOVAL_ROUTING_PENDING", fmt.Errorf("routing revocation before node removal: %w", err)}
		}
	}
	return nil
}

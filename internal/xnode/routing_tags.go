package xnode

import (
	"context"
	"crypto/sha256"
	"fmt"
)

func managedRoutingTag(identity Identity, rule string) (string, error) {
	hash := sha256.Sum256([]byte(rule))
	return namespaceTag(fmt.Sprintf("route-%x", hash[:16]), identity.Username)
}
func (s *Service) managedReservedTags(ctx context.Context, node string) (map[int64][]string, error) {
	out := map[int64][]string{}
	repo, ok := s.routingRepository()
	if !ok {
		return out, nil
	}
	plans, err := repo.XNodeRoutingPlans(ctx)
	if err != nil {
		return nil, err
	}
	for _, plan := range plans {
		doc, err := s.routingDocument(plan)
		if err != nil {
			return nil, err
		}
		identity, err := s.repo.XNodeIdentity(ctx, plan.ProxyUserID)
		if err != nil {
			return nil, err
		}
		out[identity.ProxyUserID] = []string{}
		for _, r := range doc.Choices {
			if r.InboundNodeID != node {
				continue
			}
			tag, err := managedRoutingTag(identity, r.ID)
			if err != nil {
				return nil, err
			}
			out[identity.ProxyUserID] = append(out[identity.ProxyUserID], tag)
		}
	}
	return out, nil
}

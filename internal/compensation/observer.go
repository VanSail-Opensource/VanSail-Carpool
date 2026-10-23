package compensation

import (
	"context"
	"sort"
	"time"
)

// Observe records one complete, queue-backed Proxy sample.
func (s *Service) Observe(ctx context.Context, now time.Time) error {
	config, err := s.repository.CompensationConfig(ctx)
	if err != nil {
		return err
	}
	nodes, err := s.provider.CompensationNodes(ctx)
	if err != nil {
		return err
	}
	if config.Enabled {
		squads, squadErr := s.provider.CompensationSquads(ctx)
		if squadErr != nil {
			return squadErr
		}
		mapAffectedSquads(nodes, squads)
	}
	return s.repository.RecordCompensationObservation(ctx, config, nodes, now.UTC())
}

func mapAffectedSquads(nodes []Node, squads []Squad) {
	for index := range nodes {
		active := make(map[string]struct{}, len(nodes[index].ActiveInboundUUIDs))
		for _, inbound := range nodes[index].ActiveInboundUUIDs {
			active[inbound] = struct{}{}
		}
		seen := make(map[string]struct{})
		for _, squad := range squads {
			if _, exists := seen[squad.UUID]; exists || !intersects(active, squad.Inbounds) {
				continue
			}
			seen[squad.UUID] = struct{}{}
			nodes[index].AffectedSquads = append(nodes[index].AffectedSquads, squad)
		}
		sort.Slice(nodes[index].AffectedSquads, func(a, b int) bool {
			return nodes[index].AffectedSquads[a].UUID < nodes[index].AffectedSquads[b].UUID
		})
	}
}

func intersects(active map[string]struct{}, candidates []string) bool {
	for _, candidate := range candidates {
		if _, ok := active[candidate]; ok {
			return true
		}
	}
	return false
}

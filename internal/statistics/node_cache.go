package statistics

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const nodeCacheTTL = 10 * time.Second

type nodeCache struct {
	value model.StatisticsNodesSnapshot
}

// NodeSnapshot refreshes one shared ten-second cache on page demand.
func (s *Service) NodeSnapshot(ctx context.Context, now time.Time) (model.StatisticsNodesSnapshot, error) {
	now = normalizedNow(now)
	s.nodeMu.Lock()
	defer s.nodeMu.Unlock()
	if !s.nodes.value.GeneratedAt.IsZero() && now.Sub(s.nodes.value.GeneratedAt) < nodeCacheTTL {
		return s.nodes.value, nil
	}
	nodes, err := s.provider.Nodes(ctx)
	if err != nil {
		if !s.nodes.value.GeneratedAt.IsZero() {
			stale := s.nodes.value
			stale.Stale = true
			return stale, nil
		}
		return model.StatisticsNodesSnapshot{}, err
	}
	items := make([]model.StatisticsNode, 0, len(nodes))
	for _, node := range nodes {
		items = append(items, model.StatisticsNode{UUID: node.UUID, Name: node.Name, CountryCode: node.CountryCode,
			Online: node.Online, UsersOnline: node.UsersOnline, UsersOnlineAvailable: node.UsersOnlineAvailable, RXBytesPerSecond: node.RXBytesPerSecond,
			TXBytesPerSecond: node.TXBytesPerSecond, XrayVersion: node.XrayVersion, Multiplier: node.Multiplier})
	}
	s.nodes.value = model.StatisticsNodesSnapshot{GeneratedAt: now, Nodes: items}
	return s.nodes.value, nil
}

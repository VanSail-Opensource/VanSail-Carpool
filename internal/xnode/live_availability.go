package xnode

import (
	"context"
	"slices"
	"time"
)

func (s *Service) nativeAvailability(ctx context.Context) (map[string]bool, error) {
	nodes, err := s.repo.XNodeNodes(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.repo.XNodeMigration(ctx)
	if err != nil {
		return nil, err
	}
	available := map[string]bool{}
	for _, node := range nodes {
		available[node.ID] = node.Enabled
	}
	for _, node := range state.Nodes {
		if node.Phase != "native" {
			available[node.NodeID] = false
		}
	}
	return available, nil
}

func activityAvailable(snapshot liveSnapshot, now time.Time, reportSeconds int) bool {
	return snapshot.report.Ready && snapshot.report.Synchronized && snapshot.firewallPending == "" && slices.Contains(snapshot.hello.Capabilities, "sampled_activity_v1") && freshSnapshot(snapshot, now, reportSeconds)
}

package xnode

import (
	"context"
	"fmt"
)

func (s *Service) checkRoutingDeployment(ctx context.Context, nodes []string, expected RoutingPlan) error {
	repo, ok := s.routingRepository()
	if !ok {
		return ErrUnavailable
	}
	current, err := repo.XNodeRoutingPlan(ctx, expected.ProxyUserID)
	if err != nil {
		return err
	}
	if current.OperationID != expected.OperationID || current.Revision != expected.Revision || current.Phase != expected.Phase {
		return ErrConflict
	}
	for _, node := range nodes {
		if err := s.checkSentRoutingConfig(ctx, node, current); err != nil {
			return fmt.Errorf("routing deployment node %s: %w", node, err)
		}
	}
	return nil
}

func (s *Service) checkSentRoutingConfig(ctx context.Context, node string, plan RoutingPlan) error {
	s.master.mu.RLock()
	peer := s.master.peers[node]
	s.master.mu.RUnlock()
	if peer == nil {
		return ErrUnavailable
	}
	peer.syncMu.Lock()
	defer peer.syncMu.Unlock()
	peer.mu.Lock()
	revision, digest := peer.revision, peer.digest
	peer.mu.Unlock()
	prepared := peer.wireState
	if prepared == nil || digest == "" || prepared.digest != digest {
		return ErrConflict
	}
	value, err := s.repo.XNodeNode(ctx, node)
	if err != nil {
		return err
	}
	s.master.mu.RLock()
	current := s.master.peers[node] == peer
	report := s.master.live[node].report
	s.master.mu.RUnlock()
	if !current || value.AppliedRevision < revision || report.ConfigRevision < revision || !report.Ready || !report.Synchronized {
		return ErrConflict
	}
	return s.checkRoutingConfig(ctx, node, plan, prepared.config)
}

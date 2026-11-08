package xnode

import (
	"context"
	"slices"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Synchronization uses the existing per-node queue. Completion requires the
// exact enrollment/period/paid boundary in the acknowledged native state.
func (s *Service) SyncHourlyEnrollment(ctx context.Context, enrollment string) error {
	p, err := s.hourlyPrincipal(ctx, enrollment)
	if err != nil {
		return err
	}
	if p.ProxyUserID == 0 && p.PeriodID == "" {
		return nil // Atomic dormant preparation proves no authority was issued.
	}
	if s.master == nil {
		return ErrUnavailable
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return err
	}
	sources := map[string]bool{}
	var nodes []string
	for _, group := range groups {
		if group.ID == p.GroupID && group.Enabled {
			for _, binding := range group.Bindings {
				sources[binding.NodeID] = true
				nodes = append(nodes, binding.NodeID)
			}
		}
	}
	participants, err := s.repo.XNodePeriodNodes(ctx, p.PeriodID)
	if err != nil {
		return err
	}
	for _, participant := range participants {
		nodes = append(nodes, participant.NodeID)
	}
	routing, err := s.hourlyRoutingNodes(ctx, p.OwnerID)
	if err != nil {
		return err
	}
	nodes = append(nodes, routing...)
	slices.Sort(nodes)
	nodes = slices.Compact(nodes)
	active := hourlyPrincipalAvailable(p, s.now())
	if active && len(sources) == 0 {
		return ErrUnavailable
	}
	if active {
		if err = s.ResumeHourlyOwner(ctx, p.OwnerID, p.AccountGeneration); err != nil {
			return err
		}
	}
	if err = syncDeploymentNodes(ctx, s.master, nodes); err != nil {
		return err
	}
	for _, node := range nodes {
		if err = s.checkHourlyDeployment(ctx, node, p, sources[node]); err != nil {
			return err
		}
		s.master.mu.RLock()
		peer := s.master.peers[node]
		s.master.mu.RUnlock()
		if peer != nil {
			if err = s.master.confirmFirstSync(ctx, peer); err != nil {
				return err
			}
		}
	}
	current, err := s.hourlyPrincipal(ctx, enrollment)
	if err != nil {
		return err
	}
	if current.ProxyUserID != p.ProxyUserID || current.PeriodID != p.PeriodID || current.Status != p.Status || !current.PaidThrough.Equal(p.PaidThrough) {
		return ErrConflict
	}
	return s.checkHourlyRoutingDeployment(ctx, p.OwnerID, routing)
}

func (s *Service) checkHourlyDeployment(ctx context.Context, nodeID string, expected HourlyPrincipal, source bool) error {
	s.master.mu.RLock()
	peer := s.master.peers[nodeID]
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
	if prepared == nil || digest == "" || digest != prepared.digest {
		return ErrConflict
	}
	node, err := s.repo.XNodeNode(ctx, nodeID)
	if err != nil {
		return err
	}
	s.master.mu.RLock()
	current, report := s.master.peers[nodeID] == peer, s.master.live[nodeID].report
	s.master.mu.RUnlock()
	if !current || node.AppliedRevision < revision || report.ConfigRevision < revision || !report.Ready || !report.Synchronized {
		return ErrConflict
	}
	wanted := source && hourlyPrincipalAvailable(expected, s.now())
	found := false
	for _, user := range prepared.config.Users {
		if int64(user.ID) != expected.ProxyUserID {
			continue
		}
		if !wanted || !exactHourlyUser(user, expected) {
			return ErrConflict
		}
		found = true
	}
	if wanted && !found {
		return ErrConflict
	}
	return nil
}

func exactHourlyUser(user wire.User, p HourlyPrincipal) bool {
	return user.Hourly != nil && user.Hourly.Owner.ID == p.OwnerID && user.Hourly.Owner.Generation == uint64(p.AccountGeneration) && user.Hourly.EnrollmentID == p.EnrollmentID && user.AccountingPeriod == p.PeriodID && user.ValidUntil != nil && user.ValidUntil.Equal(p.PaidThrough) && !user.BlockNew
}

func (s *Service) checkHourlyRoutingDeployment(ctx context.Context, owner string, nodes []string) error {
	repo, ok := s.routingRepository()
	if !ok || len(nodes) == 0 {
		return nil
	}
	identity, err := s.repo.XNodeIdentityByUser(ctx, owner)
	if err != nil {
		return err
	}
	plan, err := repo.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if isMissing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.checkRoutingDeployment(ctx, nodes, plan)
}

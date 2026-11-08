package xnode

import (
	"context"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
)

type HourlyReadiness struct {
	Ready                  bool
	Code                   string
	NodeIDs                []string
	CompileRevision        int64
	ObservedAt, ReadyUntil time.Time
}

// Optional owner scope includes the current routing graph's transit nodes.
// Commerce carries CompileRevision into its locked admission check.
func (s *Service) HourlySquadReadiness(ctx context.Context, groupID string, ownerID ...string) (HourlyReadiness, error) {
	result := HourlyReadiness{Code: "XNODE_HOURLY_UNAVAILABLE", ObservedAt: s.now().UTC()}
	if s.master == nil {
		return result, nil
	}
	if _, provided := s.repo.(HourlyPolicyRepository); s.hourlyPolicyProvider == nil && !provided {
		return result, nil
	}
	owner := hourly.Owner{}
	if len(ownerID) > 0 && ownerID[0] != "" {
		member, err := s.repo.UserByID(ctx, ownerID[0])
		if err != nil {
			return result, err
		}
		owner = hourly.Owner{ID: member.ID, Generation: uint64(member.AccountGeneration)}
	}
	co := s.master.hourly
	if co == nil {
		return result, nil
	}
	co.allocatorMu.Lock()
	at := s.now()
	capacity := co.allocator.CanAdmit(at, owner, co.allocatorPolicies(at))
	co.allocatorMu.Unlock()
	if !capacity {
		result.Code = "XNODE_HOURLY_CAPACITY"
		return result, nil
	}
	versioned, ok := s.repo.(compileVersionRepository)
	if !ok {
		return result, nil
	}
	version, _, err := versioned.XNodeCompileVersion(ctx, s.now())
	if err != nil {
		return result, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return result, err
	}
	for _, group := range groups {
		if group.ID == groupID && group.Enabled {
			for _, binding := range group.Bindings {
				result.NodeIDs = append(result.NodeIDs, binding.NodeID)
			}
		}
	}
	if len(result.NodeIDs) == 0 {
		return result, nil
	}
	if len(ownerID) > 0 {
		nodes, e := s.hourlyRoutingNodes(ctx, ownerID[0])
		if e != nil {
			return result, e
		}
		result.NodeIDs = append(result.NodeIDs, nodes...)
	}
	slices.Sort(result.NodeIDs)
	result.NodeIDs = slices.Compact(result.NodeIDs)
	if len(result.NodeIDs) > 128 {
		return result, nil
	}
	global, err := s.Global(ctx)
	if err != nil {
		return result, err
	}
	for _, id := range result.NodeIDs {
		node, e := s.repo.XNodeNode(ctx, id)
		if e != nil {
			return result, e
		}
		s.master.mu.RLock()
		live, peer := s.master.live[id], s.master.peers[id]
		s.master.mu.RUnlock()
		if !node.Enabled || node.RemovingAt != nil || node.RetiredAt != nil || peer == nil || !s.master.HourlyCreditAdmissionReady(id) || !paygCapabilities(live.hello.Capabilities) || !freshSnapshot(live, s.now(), global.Telemetry.ReportSeconds) || !live.report.Ready || !live.report.Synchronized || node.AppliedRevision < node.DesiredRevision || live.report.ConfigRevision < node.DesiredRevision {
			return result, nil
		}
	}
	after, _, err := versioned.XNodeCompileVersion(ctx, s.now())
	if err != nil {
		return result, err
	}
	if version != after {
		return result, ErrConflict
	}
	result.Ready, result.Code, result.CompileRevision = true, "", version
	result.ReadyUntil = result.ObservedAt.Add(4 * time.Second)
	return result, nil
}

func (s *Service) hourlyRoutingNodes(ctx context.Context, owner string) ([]string, error) {
	repo, supported := s.routingRepository()
	if !supported || owner == "" {
		return nil, nil
	}
	identity, err := s.repo.XNodeIdentityByUser(ctx, owner)
	if isMissing(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plan, err := repo.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if isMissing(err) || err == nil && (plan.Phase == "disabled" || plan.Phase == "revoking") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.routingPlanNodes(plan)
}

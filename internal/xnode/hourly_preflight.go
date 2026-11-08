package xnode

import (
	"context"
	"slices"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// The preview is local validation only. Its synthetic future boundary and
// BlockNew user never enter a transport queue or any financial transaction.
func (s *Service) PreflightHourlyEnrollment(ctx context.Context, enrollment string) (HourlyReadiness, error) {
	p, err := s.hourlyPrincipal(ctx, enrollment)
	if err != nil {
		return HourlyReadiness{}, err
	}
	if p.Status != "preparing" || p.ProxyUserID <= 0 || p.PeriodID == "" {
		return HourlyReadiness{}, ErrConflict
	}
	ready, err := s.HourlySquadReadiness(ctx, p.GroupID, p.OwnerID)
	if err != nil || !ready.Ready {
		return ready, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return ready, err
	}
	sources := map[string]bool{}
	for _, group := range groups {
		if group.ID == p.GroupID {
			for _, binding := range group.Bindings {
				sources[binding.NodeID] = true
			}
		}
	}
	for _, node := range ready.NodeIDs {
		preview := s.withCompileReads(node)
		preview.hourlyPreview = enrollment
		config, e := preview.Compile(ctx, node)
		if e != nil {
			return ready, e
		}
		s.master.mu.RLock()
		capabilities := s.master.live[node].hello.Capabilities
		s.master.mu.RUnlock()
		if e = configurationCapabilities(config, capabilities); e != nil {
			return ready, e
		}
		if slices.ContainsFunc(config.Users, func(user wire.User) bool { return int64(user.ID) == p.ProxyUserID && !user.BlockNew }) {
			return ready, ErrConflict
		}
		if sources[node] && !slices.ContainsFunc(config.Users, func(user wire.User) bool {
			return int64(user.ID) == p.ProxyUserID && user.Hourly != nil && user.BlockNew && user.AccountingPeriod == p.PeriodID
		}) {
			return ready, ErrConflict
		}
	}
	current, err := s.HourlySquadReadiness(ctx, p.GroupID, p.OwnerID)
	if err != nil || current.CompileRevision != ready.CompileRevision {
		return current, ErrConflict
	}
	return current, nil
}

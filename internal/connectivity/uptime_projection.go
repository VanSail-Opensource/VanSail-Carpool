package connectivity

import (
	"context"
	"time"
)

type timelineRepository interface {
	ConnectivityTimeline(context.Context, string, time.Time) ([]Attempt, error)
}

// ProjectMember derives uptime without exposing the monitoring account or diagnostics.
func (s *Service) ProjectMember(ctx context.Context, subscription *MemberSubscription, now time.Time) {
	for index := range subscription.Hosts {
		subscription.Hosts[index].Timeline = UnknownTimeline(now)
	}
	subscription.Summary = UptimeSummary{ActiveCombo: subscription.ActiveCombo, UptimeTimeline: UnknownTimeline(now)}
	if s == nil {
		subscription.Summary.ErrorCode = "CONNECTIVITY_NOT_CONFIGURED"
	} else {
		cfg, err := s.readConfig(ctx)
		if err == nil && cfg.ScheduledEnabled {
			subscription.Summary.MonitoringEnabled = true
			err = s.projectHostTimelines(ctx, subscription.Hosts, now, cfg)
		}
		if err != nil {
			if ErrorCode(err) == "CONNECTIVITY_INTERRUPTED" {
				subscription.Summary.MonitoringEnabled = false
			}
			for index := range subscription.Hosts {
				subscription.Hosts[index].Timeline = UnknownTimeline(now)
			}
			subscription.Summary.ErrorCode = ErrorCode(err)
		}
	}
	byHost := make(map[string]UptimeTimeline, len(subscription.Hosts))
	for _, host := range subscription.Hosts {
		byHost[host.UUID] = host.Timeline
	}
	squadTimelines := make([]UptimeTimeline, 0, len(subscription.Squads))
	for index := range subscription.Squads {
		squad := &subscription.Squads[index]
		timelines := make([]UptimeTimeline, 0, len(squad.HostUUIDs))
		for _, uuid := range squad.HostUUIDs {
			if timeline, exists := byHost[uuid]; exists {
				timelines = append(timelines, timeline)
			} else {
				timelines = append(timelines, UnknownTimeline(now))
			}
		}
		squad.Timeline = AggregateTimelines(timelines, now)
		squadTimelines = append(squadTimelines, squad.Timeline)
	}
	subscription.Summary.UptimeTimeline = AggregateTimelines(squadTimelines, now)
}

func (s *Service) projectHostTimelines(ctx context.Context, hosts []SubscriptionHost, now time.Time, cfg Config) error {
	if cfg.RemnawaveUserID == 0 {
		return &CodeError{Code: "CONNECTIVITY_NOT_CONFIGURED"}
	}
	repository, ok := s.repository.(timelineRepository)
	if !ok {
		return &CodeError{Code: "CONNECTIVITY_STORAGE_UNAVAILABLE"}
	}
	items, err := repository.ConnectivityTimeline(ctx, ConfigHash(cfg), now)
	if err != nil {
		return &CodeError{Code: "CONNECTIVITY_STORAGE_UNAVAILABLE", Err: err}
	}
	byHost := map[string][]Attempt{}
	runHosts := map[string]map[string]bool{}
	s.mu.RLock()
	hostCount := 0
	if s.loadHash == ConfigHash(cfg) {
		hostCount = len(s.targets)
	}
	s.mu.RUnlock()
	for _, item := range items {
		byHost[item.HostUUID] = append(byHost[item.HostUUID], item)
		if runHosts[item.RunID] == nil {
			runHosts[item.RunID] = map[string]bool{}
		}
		runHosts[item.RunID][item.HostUUID] = true
		if len(runHosts[item.RunID]) > hostCount {
			hostCount = len(runHosts[item.RunID])
		}
	}
	// Scheduling waits until the whole serial batch finishes before its interval.
	freshness := 2*time.Duration(cfg.IntervalSeconds)*time.Second + time.Duration(hostCount)*cfg.hostCheckBudget()
	for index := range hosts {
		hosts[index].Timeline = HostTimeline(byHost[hosts[index].UUID], now, freshness)
	}
	// Discard a projection if monitoring configuration changed during its query.
	current, err := s.readConfig(ctx)
	if err != nil {
		return err
	}
	if ConfigHash(current) != ConfigHash(cfg) {
		for index := range hosts {
			hosts[index].Timeline = UnknownTimeline(now)
		}
		return &CodeError{Code: "CONNECTIVITY_INTERRUPTED"}
	}
	return nil
}

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
	available := map[string]bool{}
	for _, node := range nodes {
		available[node.ID] = node.Enabled
	}
	return available, nil
}

func activityAvailable(snapshot liveSnapshot, now time.Time, reportSeconds int, sampleSeconds ...int) bool {
	if !snapshot.report.Ready || !snapshot.report.Synchronized || snapshot.firewallPending != "" || !slices.Contains(snapshot.hello.Capabilities, "sampled_activity_v1") || !freshSnapshot(snapshot, now, reportSeconds) {
		return false
	}
	if snapshot.activityReports && snapshot.report.Users.SampledAt == nil {
		return false
	}
	return now.Before(activityExpiry(snapshot, reportSeconds, sampleSeconds...))
}

func activityExpiry(snapshot liveSnapshot, reportSeconds int, sampleSeconds ...int) time.Time {
	expiry := snapshot.receivedAt.Add(reportWindow(reportSeconds))
	if snapshot.report.Users.SampledAt != nil {
		seconds := reportSeconds
		if len(sampleSeconds) > 0 {
			seconds = max(seconds, sampleSeconds[0])
		}
		// Compare freshness in master time. Agent wall clocks can drift while
		// the authenticated report stream and sample sequence remain current.
		sampledAt := snapshot.activitySampleReceivedAt
		if sampledAt.IsZero() {
			// Snapshots without a receive marker have no evidence that this
			// sample advanced during the current report stream. Preserve the
			// sample's own age instead of refreshing it from report receipt.
			sampledAt = *snapshot.report.Users.SampledAt
		}
		deadline := sampledAt.Add(reportWindow(seconds))
		if deadline.Before(expiry) {
			expiry = deadline
		}
	}
	return expiry
}

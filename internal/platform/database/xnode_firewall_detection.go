package database

import (
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type firewallDetectionWindow struct {
	start, time time.Time
	count       uint64
	rule        wire.FirewallRule
	banSeconds  int
	reported    bool
}

// Detection windows are bounded, volatile source-IP observations. Durable event
// cursors deduplicate retries; only an authorized ban becomes persistent state.
func (s *Store) stageDetection(event wire.FirewallEvent, rule wire.FirewallRule, policy wire.FirewallPolicy, now time.Time, staged map[string]firewallDetectionWindow) (wire.FirewallEvent, bool, error) {
	if event.Count == 0 || event.Time.IsZero() || event.FirstSeen.IsZero() || event.FirstSeen.After(event.Time) || event.Time.After(now.Add(5*time.Minute)) {
		return event, false, xnode.ErrInvalid
	}
	duration := time.Duration(rule.WindowSeconds) * time.Second
	if !event.Time.Add(duration).After(now) {
		return event, false, nil
	}
	key := event.IP + "|" + event.Rule
	window, ok := staged[key]
	if !ok {
		window, ok = s.firewallWindows[key]
	}
	if !ok && len(s.firewallWindows)+len(staged) >= 65536 {
		return event, false, nil
	}
	if !ok || window.rule != rule || window.banSeconds != policy.BanSeconds || !event.Time.Before(window.start.Add(duration)) {
		window = firewallDetectionWindow{start: event.FirstSeen, rule: rule, banSeconds: policy.BanSeconds}
	}
	if event.Time.Before(window.start) {
		if !window.time.Before(event.FirstSeen.Add(duration)) {
			return event, false, nil
		}
		window.start = event.FirstSeen
	}
	if window.reported {
		return event, false, nil
	}
	if math.MaxUint64-window.count < event.Count {
		return event, false, xnode.ErrInvalid
	}
	window.count += event.Count
	if event.Time.After(window.time) {
		window.time = event.Time
	}
	ban := window.count >= uint64(rule.Threshold)
	window.reported = ban
	staged[key] = window
	event.Count, event.FirstSeen, event.Time = window.count, window.start, window.time
	return event, ban, nil
}

func (s *Store) ResetFirewallDetection() {
	s.firewallDetectionMu.Lock()
	defer s.firewallDetectionMu.Unlock()
	s.firewallWindows = nil
}

package xnode

import (
	"context"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type automaticLeaseBanRepository interface {
	ReleaseAutomaticLeaseBans(context.Context, []AutomaticLeaseGrant, time.Time) error
}

func (m *Master) releaseAutomaticLeaseBans(ctx context.Context) (bool, error) {
	repo, ok := m.service.repo.(automaticLeaseBanRepository)
	if !ok {
		return false, nil
	}
	items := m.automaticLeases.pendingBanReleases(m.service.Now())
	if len(items) == 0 {
		return false, nil
	}
	if err := repo.ReleaseAutomaticLeaseBans(ctx, items, m.service.Now()); err != nil {
		return false, err
	}
	m.automaticLeases.acknowledgeBanReleases(items)
	m.NotifyFirewallChanged()
	return true, nil
}

func (m *automaticLeaseManager) protectedEventIPs(events []wire.FirewallEvent, now time.Time, recheck bool) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ips := map[string]bool{}
	for _, event := range events {
		if event.Kind != "detected" {
			continue
		}
		ip, err := firewallPublicAddr(event.IP)
		if err != nil {
			continue
		}
		if lease := m.leases[ip]; lease != nil && lease.until.After(now) {
			ips[ip.String()] = true
			if recheck {
				m.pendingBans[ip] = lease.generation
			}
		}
	}
	result := make([]string, 0, len(ips))
	for ip := range ips {
		result = append(result, ip)
	}
	return result
}

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
	grants := make([]AutomaticLeaseGrant, len(items))
	for i, item := range items {
		grants[i] = item.grant
	}
	if err := repo.ReleaseAutomaticLeaseBans(ctx, grants, m.service.Now()); err != nil {
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
				m.queueBanReleaseLocked(lease)
			}
		}
	}
	result := make([]string, 0, len(ips))
	for ip := range ips {
		result = append(result, ip)
	}
	return result
}

// A late event can need another release without renewing the lease itself.
// Each immutable intent therefore has its own identity, independent of the
// delivery generation, so an older SQL completion cannot erase newer work.
func (m *automaticLeaseManager) queueBanReleaseLocked(item *automaticLease) {
	m.pendingBans[item.ip] = &automaticBanRelease{grant: AutomaticLeaseGrant{IP: item.ip.String(), Generation: item.generation, Until: item.until}}
}

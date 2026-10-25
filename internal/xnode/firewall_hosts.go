package xnode

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type hostOrigin struct {
	addresses  []string
	refreshAt  time.Time
	validUntil time.Time
}

// DNS identifies current exact origin addresses, never ownership. Only addresses
// corroborated by the associated agent or administrator-owned DDNS state qualify.
// In particular, a subscription hostname cannot whitelist a shared CDN edge.
func verifiedHostOrigins(addresses, trusted []string) []string {
	result := []string{}
	for _, raw := range addresses {
		if ip, err := FirewallPublicIP(raw); err == nil && slices.Contains(trusted, ip) {
			result = append(result, ip)
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func (s *Service) resolveHostOrigins(ctx context.Context, host string) ([]string, error) {
	return upstreamqueue.Do(ctx, s.validationQueue, func(ctx context.Context) ([]string, error) {
		child, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		answers, err := net.DefaultResolver.LookupNetIP(child, "ip", host)
		if err != nil {
			return nil, err
		}
		addresses := []string{}
		for _, answer := range answers {
			if ip, err := FirewallPublicIP(answer.Unmap().String()); err == nil {
				addresses = append(addresses, ip)
			}
		}
		return addresses, nil
	})
}

func (m *Master) refreshHostOrigins(ctx context.Context) error {
	endpoints, err := m.service.repo.XNodeEndpoints(ctx)
	if err != nil {
		return err
	}
	nodes, err := m.service.Nodes(ctx)
	if err != nil {
		return err
	}
	trusted := map[string][]string{}
	m.mu.RLock()
	for id, ips := range m.managedNodeIPs {
		trusted[id] = slices.Clone(ips)
	}
	for id, ips := range m.ddnsNodeIPs {
		trusted[id] = append(trusted[id], ips...)
	}
	m.mu.RUnlock()
	for _, node := range nodes {
		_, settings, err := m.service.Node(ctx, node.ID)
		if err != nil {
			return err
		}
		for _, job := range settings.DDNS {
			for _, raw := range []string{job.IPv4, job.IPv6} {
				if ip, err := FirewallPublicIP(raw); err == nil {
					trusted[node.ID] = append(trusted[node.ID], ip)
				}
			}
		}
	}
	hosts := map[string][]string{}
	for _, endpoint := range endpoints {
		if !endpoint.Enabled || len(trusted[endpoint.NodeID]) == 0 {
			continue
		}
		host := strings.ToLower(strings.TrimSuffix(endpoint.Address, "."))
		if _, err := netip.ParseAddr(host); err == nil {
			continue
		}
		hosts[host] = append(hosts[host], trusted[endpoint.NodeID]...)
	}
	if m.hostOrigins == nil {
		m.hostOrigins = map[string]hostOrigin{}
	}
	for host := range m.hostOrigins {
		if _, ok := hosts[host]; !ok {
			delete(m.hostOrigins, host)
		}
	}
	// Refresh oldest work first, with at most two queued DNS requests per tick.
	keys := make([]string, 0, len(hosts))
	for host := range hosts {
		keys = append(keys, host)
	}
	slices.SortFunc(keys, func(a, b string) int { return m.hostOrigins[a].refreshAt.Compare(m.hostOrigins[b].refreshAt) })
	now, budget := m.service.now(), 2
	protected := []string{}
	for _, host := range keys {
		entry := m.hostOrigins[host]
		if !entry.refreshAt.After(now) && budget > 0 && (len(m.hostOrigins) < 4096 || !entry.refreshAt.IsZero()) {
			budget--
			answers, err := m.service.resolveHostOrigins(ctx, host)
			entry.refreshAt = now.Add(time.Minute)
			if err == nil {
				entry.addresses = answers
				entry.refreshAt = now.Add(5 * time.Minute)
				entry.validUntil = now.Add(15 * time.Minute)
			}
			m.hostOrigins[host] = entry
		}
		if entry.validUntil.After(now) {
			protected = append(protected, verifiedHostOrigins(entry.addresses, hosts[host])...)
		}
	}
	slices.Sort(protected)
	m.mu.Lock()
	m.managedHostIPs = slices.Compact(protected)
	m.mu.Unlock()
	return nil
}

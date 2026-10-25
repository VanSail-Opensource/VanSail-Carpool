package xnode

import (
	"context"
	"maps"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func ddnsHostname(job wire.DDNSJob) string {
	if job.Provider == wire.BuiltinDDNSProvider {
		return ""
	}
	zone := strings.ToLower(strings.TrimSuffix(job.Zone, "."))
	name := strings.ToLower(strings.TrimSuffix(job.Name, "."))
	if name == "@" {
		name = zone
	}
	if zone == "" || name != zone && !strings.HasSuffix(name, "."+zone) {
		return ""
	}
	return name
}
func ddnsOriginKey(node, host string) string {
	return node + "\n" + strings.ToLower(strings.TrimSuffix(host, "."))
}

// A successful external-provider event is authenticated origin evidence, not
// an arbitrary recursive DNS answer. Scope it to that node's published hostname
// and preserve the hostname itself for client TLS/SNI and member choices.
func (s *Service) publishDDNSOrigins(ctx context.Context, states []DDNSChangeState, jobs map[string]map[string]wire.DDNSJob) error {
	if len(states) == 0 {
		s.routingRuntime.mu.Lock()
		empty := len(s.routingRuntime.ddns) == 0
		s.routingRuntime.mu.Unlock()
		if empty {
			return nil
		}
	}
	hosts, err := s.repo.XNodeEndpoints(ctx)
	if err != nil {
		return err
	}
	eligible := map[string]bool{}
	for _, host := range hosts {
		eligible[ddnsOriginKey(host.NodeID, host.Address)] = true
	}
	origins := map[string]string{}
	conflicts := map[string]bool{}
	for _, state := range states {
		if state.Phase != "routes" && state.Phase != "settled" {
			continue
		}
		job, ok := jobs[state.NodeID][state.Change.JobID]
		if !ok || job.Generation != state.Change.Generation {
			continue
		}
		host := ddnsHostname(job)
		key := ddnsOriginKey(state.NodeID, host)
		if host == "" || !eligible[key] || conflicts[key] {
			continue
		}
		address := state.Change.IPv4
		if address == "" {
			address = state.Change.IPv6
		}
		ip, err := FirewallPublicIP(address)
		if err != nil {
			continue
		}
		if prior := origins[key]; prior != "" && prior != ip {
			delete(origins, key)
			conflicts[key] = true
			continue
		}
		origins[key] = ip
	}
	r := s.routingRuntime
	r.mu.Lock()
	if maps.Equal(r.ddns, origins) {
		r.mu.Unlock()
		return nil
	}
	changedHosts := map[string]bool{}
	for key := range r.ddns {
		_, host, _ := strings.Cut(key, "\n")
		changedHosts[host] = true
	}
	for key := range origins {
		_, host, _ := strings.Cut(key, "\n")
		changedHosts[host] = true
	}
	for host := range r.peers {
		if changedHosts[strings.ToLower(strings.TrimSuffix(host, "."))] {
			delete(r.peers, host)
		}
	}
	r.ddns = origins
	r.checked = false
	r.mu.Unlock()
	s.localCompileRevision.Add(1)
	return nil
}

func (s *Service) ddnsRoutingPeer(node, host string) string {
	r := s.routingRuntime
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ddns[ddnsOriginKey(node, host)]
}

func (s *Service) previousDDNSOrigin(node string, job wire.DDNSJob) []string {
	host := ddnsHostname(job)
	if host == "" {
		return nil
	}
	r := s.routingRuntime
	r.mu.Lock()
	defer r.mu.Unlock()
	ips := []string{}
	for _, address := range []string{r.ddns[ddnsOriginKey(node, host)], r.peers[host].address} {
		if ip, err := FirewallPublicIP(address); err == nil {
			ips = append(ips, ip)
		}
	}
	return ips
}

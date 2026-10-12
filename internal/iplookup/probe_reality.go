package iplookup

import (
	"context"
	"crypto/x509"
	"net/netip"
	"strings"

	"github.com/miekg/dns"
)

func (s *EnhancedService) realityDNS(ctx context.Context, ip string, cert *x509.Certificate, c Config) ProbeFinding {
	addr, _ := netip.ParseAddr(ip)
	family := uint16(dns.TypeA)
	if addr.Is6() {
		family = dns.TypeAAAA
	}
	names := append(append([]string{}, cert.DNSNames...), cert.Subject.CommonName)
	seen := map[string]bool{}
	best := ProbeFinding{Protocol: "reality", Confidence: "observed", Evidence: "dns_no_address"}
	for _, name := range names {
		if strings.Contains(name, "*") {
			continue
		}
		domain := endpointHost(name)
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		resolved := s.resolveDNS(ctx, domain, c.DNSResolvers, family, true, true)
		finding := classifyReality(ip, domain, resolved.observations, len(c.DNSResolvers))
		if finding.Protocol == "web_server" {
			return finding
		}
		if finding.Confidence == "suspected" || best.Confidence != "suspected" {
			best = finding
		}
	}
	return best
}

func classifyReality(ip, domain string, observations []DNSObservation, configured int) ProbeFinding {
	f := ProbeFinding{Protocol: "reality", Confidence: "observed", Evidence: "dns_no_address", DNS: &ProbeDNS{Domain: domain, Resolvers: observations}}
	usable, negative, failed := 0, 0, 0
	seen := map[string]bool{}
	for _, obs := range observations {
		if seen[obs.Server] {
			continue
		}
		seen[obs.Server] = true
		if obs.Status == "success" {
			usable++
			for _, address := range obs.Addresses {
				if address == ip {
					f.Protocol = "web_server"
					f.Evidence = "dns_matches_ip"
					return f
				}
			}
		} else if obs.Status == "no_address" {
			negative++
		} else {
			failed++
		}
	}
	if usable >= min(2, configured) && usable > 0 {
		f.Confidence = "suspected"
		f.Evidence = "feasible_tls_dns_mismatch"
	} else if usable > 0 || (negative == 0 && failed > 0) {
		f.Confidence = "unavailable"
		f.Evidence = "dns_unavailable"
	}
	return f
}

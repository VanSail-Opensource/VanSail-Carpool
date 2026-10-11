package iplookup

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"strings"
	"time"
)

func randomRead(b []byte) (int, error) { return rand.Read(b) }
func (s *EnhancedService) probeTLS(ctx context.Context, ip string, port int, c Config) probeOutcome {
	out := probeOutcome{status: "success"}
	conn, err := probeDial(ctx, ip, port)
	if err != nil {
		out.status = "error"
		return out
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	// This scanner observes certificates and does not authenticate a remote service.
	client := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h2", "http/1.1"}})
	if err := client.HandshakeContext(ctx); err != nil {
		return out
	}
	out.tls = true
	out.findings = []ProbeFinding{{Protocol: "tls", Confidence: "observed", Evidence: "handshake"}}
	state := client.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return out
	}
	cert := state.PeerCertificates[0]
	if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != "h2" || cert.Subject.CommonName == "" || len(cert.Issuer.Organization) == 0 {
		return out
	}
	domain := cert.Subject.CommonName
	if strings.HasPrefix(domain, "*.") {
		for _, name := range cert.DNSNames {
			if !strings.HasPrefix(name, "*.") {
				domain = name
				break
			}
		}
	}
	if strings.HasPrefix(domain, "*.") {
		out.status = "error"
		out.findings = append(out.findings, ProbeFinding{Protocol: "reality", Confidence: "unavailable", Evidence: "dns_unavailable"})
		return out
	}
	ips, err := s.resolve(ctx, domain, c.DNSResolvers, true)
	if err != nil {
		out.status = "error"
		out.findings = append(out.findings, ProbeFinding{Protocol: "reality", Confidence: "unavailable", Evidence: "dns_unavailable"})
		return out
	}
	for _, address := range ips {
		if address.String() == ip {
			return out
		}
	}
	out.findings = append(out.findings, ProbeFinding{Protocol: "reality", Confidence: "suspected", Evidence: "feasible_tls_dns_mismatch"})
	return out
}

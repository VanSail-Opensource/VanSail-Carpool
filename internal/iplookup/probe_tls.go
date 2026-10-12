package iplookup

import (
	"context"
	"crypto/rand"
	"crypto/tls"
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
	web := false
	if len(state.PeerCertificates) > 0 && state.Version == tls.VersionTLS13 && state.NegotiatedProtocol == "h2" {
		finding := s.realityDNS(ctx, ip, state.PeerCertificates[0], c)
		out.findings = append(out.findings, finding)
		web = finding.Protocol == "web_server"
		if finding.Confidence == "unavailable" {
			out.status = "error"
		}
	}
	out.findings = append(out.findings, probeWeb(ctx, ip, port, "https", web)...)
	return out
}

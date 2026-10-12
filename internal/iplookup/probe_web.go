package iplookup

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"
)

// probeWeb observes ordinary HTTP independently of the named Cloudflare trace.
// Its caller must already be executing in the existing probes queue.
func probeWeb(ctx context.Context, ip string, port int, scheme string, observed bool) []ProbeFinding {
	findings := []ProbeFinding{}
	client, closeClient := webProbeClient(ip, port, "")
	defer closeClient()
	if observed {
		findings = append(findings, ProbeFinding{Protocol: "web_server", Confidence: "observed", Evidence: "http_response"})
	} else if client != nil {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+net.JoinHostPort(ip, strconv.Itoa(port))+"/", nil)
		if err == nil {
			request.Header.Set("User-Agent", "TX-Carpool-IP-Probe")
			if response, err := client.Do(request); err == nil {
				_ = response.Body.Close()
				findings = append(findings, ProbeFinding{Protocol: "web_server", Confidence: "observed", Evidence: "http_response"})
			}
		}
	}
	return append(findings, probeCloudflare(ctx, ip, port, scheme).findings...)
}

// webProbeClient pins every connection to the observed IP, with no proxy or redirects.
func webProbeClient(ip string, port int, serverName string) (*http.Client, func()) {
	address, err := netip.ParseAddr(ip)
	if err != nil || address.Zone() != "" || port < 1 || port > 65535 {
		return nil, func() {}
	}
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", net.JoinHostPort(address.String(), strconv.Itoa(port)))
		},
		// These probes observe remote services without authenticating certificates.
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true, ServerName: serverName},
		TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 4 * time.Second,
		MaxResponseHeaderBytes: 8192, DisableKeepAlives: true,
	}
	return &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, transport.CloseIdleConnections
}

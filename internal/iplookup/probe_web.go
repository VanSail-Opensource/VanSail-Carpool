package iplookup

import (
	"context"
	"crypto/tls"
	"io"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// probeWeb runs inside the existing probes executor and never follows redirects.
func probeWeb(ctx context.Context, ip string, port int, scheme string, observed bool) []ProbeFinding {
	address, err := netip.ParseAddr(ip)
	if err != nil || address.Zone() != "" || port < 1 || port > 65535 || (scheme != "http" && scheme != "https") {
		return nil
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: 4 * time.Second}).DialContext,
		// As with TLS probes, observe the endpoint without authenticating its certificate.
		TLSClientConfig:        &tls.Config{InsecureSkipVerify: true},
		TLSHandshakeTimeout:    4 * time.Second,
		ResponseHeaderTimeout:  4 * time.Second,
		MaxResponseHeaderBytes: 8192,
		DisableKeepAlives:      true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	base := scheme + "://" + net.JoinHostPort(address.String(), strconv.Itoa(port))
	get := func(path string) (*http.Response, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("User-Agent", "TX-Carpool-IP-Probe")
		return client.Do(request)
	}
	if !observed {
		response, err := get("/")
		if err != nil {
			return nil
		}
		_ = response.Body.Close()
	}
	findings := []ProbeFinding{{Protocol: "web_server", Confidence: "observed", Evidence: "http_response"}}
	response, err := get("/cdn-cgi/trace")
	if err != nil {
		return findings
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return findings
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8193))
	if err != nil || len(body) > 8192 {
		return findings
	}
	if colo := cloudflareTrace(body, scheme); colo != "" {
		findings = append(findings, ProbeFinding{Protocol: "cloudflare", Confidence: "confirmed", Evidence: "cdn_trace:" + colo})
	}
	return findings
}

// cloudflareTrace validates the trace signature; ip is the caller, not the probed IP.
func cloudflareTrace(body []byte, scheme string) string {
	fields := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			return ""
		}
		if _, exists := fields[key]; exists {
			return ""
		}
		fields[key] = value
	}
	address, err := netip.ParseAddr(fields["ip"])
	stamp, stampErr := strconv.ParseFloat(fields["ts"], 64)
	colo := fields["colo"]
	if err != nil || address.Zone() != "" || stampErr != nil || stamp <= 0 || math.IsNaN(stamp) || math.IsInf(stamp, 0) || fields["fl"] == "" || fields["h"] == "" || fields["http"] == "" || fields["visit_scheme"] != scheme || len(colo) != 3 {
		return ""
	}
	for _, c := range colo {
		if c < 'A' || c > 'Z' {
			return ""
		}
	}
	return colo
}

package iplookup

import (
	"context"
	"io"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

const cloudflareProbeHost = "cp.cloudflare.com"

// probeCloudflare sends the named trace to the tested IP inside the probes executor.
func probeCloudflare(ctx context.Context, ip string, port int, scheme string) probeOutcome {
	out := probeOutcome{status: "success", result: "not_detected"}
	if scheme != "http" && scheme != "https" {
		return probeOutcome{status: "error", result: "unavailable"}
	}
	client, closeClient := webProbeClient(ip, port, cloudflareProbeHost)
	defer closeClient()
	if client == nil {
		return probeOutcome{status: "error", result: "unavailable"}
	}
	url := scheme + "://" + net.JoinHostPort(cloudflareProbeHost, strconv.Itoa(port)) + "/cdn-cgi/trace"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return probeOutcome{status: "error", result: "unavailable"}
	}
	request.Host = cloudflareProbeHost
	request.Header.Set("User-Agent", "TX-Carpool-IP-Probe")
	response, err := client.Do(request)
	if err != nil {
		return probeOutcome{status: "error", result: "unavailable"}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return out
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8193))
	if err != nil || len(body) > 8192 {
		return probeOutcome{status: "error", result: "unavailable"}
	}
	if colo := cloudflareTrace(body, scheme); colo != "" {
		out.result = "detected"
		out.findings = []ProbeFinding{{Protocol: "cloudflare", Confidence: "confirmed", Evidence: "cdn_trace:" + colo}}
	}
	return out
}

// cloudflareTrace validates the signature; ip is the caller, not the probed IP.
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
	if err != nil || address.Zone() != "" || stampErr != nil || stamp <= 0 || math.IsNaN(stamp) || math.IsInf(stamp, 0) || fields["fl"] == "" || fields["h"] != cloudflareProbeHost || fields["http"] == "" || fields["visit_scheme"] != scheme || len(colo) != 3 {
		return ""
	}
	for _, c := range colo {
		if c < 'A' || c > 'Z' {
			return ""
		}
	}
	return colo
}

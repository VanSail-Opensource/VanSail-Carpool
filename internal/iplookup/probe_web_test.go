package iplookup

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

const validCloudflareTrace = "fl=abc123\nh=cp.cloudflare.com\nip=8.8.4.4\nts=1700000000.5\nvisit_scheme=http\nhttp=http/1.1\ncolo=SJC\n"

func TestCloudflareTraceSignature(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"trace with trailing newline", validCloudflareTrace, true},
		{"HTML mentions colo", "<html>colo=SJC</html>", false},
		{"colo only", "colo=SJC\n", false},
		{"invalid caller", strings.ReplaceAll(validCloudflareTrace, "8.8.4.4", "unknown"), false},
		{"invalid timestamp", strings.ReplaceAll(validCloudflareTrace, "1700000000.5", "NaN"), false},
		{"wrong scheme", strings.ReplaceAll(validCloudflareTrace, "visit_scheme=http", "visit_scheme=https"), false},
		{"duplicate fields", validCloudflareTrace + "colo=LHR\n", false},
		{"bad colo", strings.ReplaceAll(validCloudflareTrace, "SJC", "sjc"), false},
		{"wrong host", strings.ReplaceAll(validCloudflareTrace, "cp.cloudflare.com", "example.net"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cloudflareTrace([]byte(tc.body), "http") != ""; got != tc.valid {
				t.Fatalf("trace validation=%v", got)
			}
		})
	}
}

func TestQueuedWebProbeCloudflareAndRedirectBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name          string
		tls, detected bool
		status        int
		body          string
	}{
		{"HTTP trace", false, true, 200, validCloudflareTrace},
		{"HTTPS trace", true, true, 200, strings.ReplaceAll(validCloudflareTrace, "visit_scheme=http", "visit_scheme=https")},
		{"ordinary web server", false, false, 404, "missing"},
		{"header alone", false, false, 200, "not a trace"},
		{"redirect", false, false, 302, validCloudflareTrace},
		{"oversized", false, false, 200, validCloudflareTrace + strings.Repeat("x", 8192)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var traceCalls, redirects atomic.Int32
			redirect := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirects.Add(1) }))
			defer redirect.Close()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/cdn-cgi/trace" {
					if r.Host != cloudflareProbeHost || (tc.tls && r.TLS.ServerName != cloudflareProbeHost) {
						t.Errorf("trace host/SNI not pinned: %q %+v", r.Host, r.TLS)
					}
					traceCalls.Add(1)
					w.Header().Set("Server", "cloudflare")
					w.Header().Set("Location", redirect.URL)
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
					return
				}
				w.WriteHeader(http.StatusMethodNotAllowed)
			})
			server := httptest.NewServer(handler)
			if tc.tls {
				server.Close()
				server = httptest.NewTLSServer(handler)
			}
			defer server.Close()
			ip, rawPort, err := net.SplitHostPort(server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.Atoi(rawPort)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "probes", Capacity: 2})
			if err != nil {
				t.Fatal(err)
			}
			if err := queue.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = queue.Shutdown(context.Background()) }()
			s := NewEnhancedService(nil, map[string]*upstreamqueue.Queue{"probes": queue}, nil)
			key := "http"
			if tc.tls {
				key = "tls"
			}
			out, err := upstreamqueue.Do(ctx, queue, func(callCtx context.Context) (probeOutcome, error) {
				return s.probe(callCtx, ip, port, key, Config{}, func(int) error { return nil }), nil
			})
			if err != nil {
				t.Fatal(err)
			}
			web, detected := false, false
			for _, finding := range out.findings {
				web = web || finding.Protocol == "web_server"
				detected = detected || finding.Protocol == "cloudflare" && finding.Confidence == "confirmed"
			}
			if !web || detected != tc.detected || traceCalls.Load() != 1 || redirects.Load() != 0 {
				t.Fatalf("wrong web evidence: %+v traces=%d redirects=%d", out, traceCalls.Load(), redirects.Load())
			}
		})
	}
}

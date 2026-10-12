package iplookup

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

func TestNamedCloudflareTraceSurvivesUnnamedTLSRejection(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != cloudflareProbeHost || r.TLS.ServerName != cloudflareProbeHost || r.URL.Path != "/cdn-cgi/trace" {
			t.Errorf("wrong named request: %q %q %s", r.Host, r.TLS.ServerName, r.URL.Path)
		}
		_, _ = w.Write([]byte(strings.ReplaceAll(validCloudflareTrace, "visit_scheme=http", "visit_scheme=https")))
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		if hello.ServerName != cloudflareProbeHost {
			return nil, errors.New("named handshake required")
		}
		return nil, nil
	}}
	server.StartTLS()
	defer server.Close()
	ip, number, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(number)
	if err != nil {
		t.Fatal(err)
	}
	s := subnetProbeService(t)
	out, err := upstreamqueue.Do(context.Background(), s.queues["probes"], func(ctx context.Context) (probeOutcome, error) {
		return s.probeTLS(ctx, ip, port, Config{}), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	confirmed := false
	for _, finding := range out.findings {
		confirmed = confirmed || finding.Protocol == "cloudflare" && finding.Confidence == "confirmed"
		if finding.Protocol == "reality" {
			t.Fatal("named Cloudflare certificate became Reality evidence")
		}
	}
	if !confirmed || out.tls {
		t.Fatalf("named trace not isolated from rejected TLS: %+v", out)
	}
}

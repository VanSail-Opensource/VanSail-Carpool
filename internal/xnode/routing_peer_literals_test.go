package xnode

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

// These tests replace the process resolver and therefore remain nonparallel.
func routingLiteralTestService(t *testing.T) (*Service, *atomic.Int32) {
	t.Helper()
	calls := new(atomic.Int32)
	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		calls.Add(1)
		return nil, errors.New("fixture DNS unavailable")
	}}
	t.Cleanup(func() { net.DefaultResolver = previous })
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "routing-literal-fixture", Capacity: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err = queue.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := queue.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return NewService(nil, nil, nil, nil, queue), calls
}

func TestRoutingLiteralIPsHaveNoDNSDeadline(t *testing.T) {
	service, calls := routingLiteralTestService(t)
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	service.now = func() time.Time { return now }
	for _, tc := range []struct{ address, want string }{
		{"8.8.8.8", "8.8.8.8"},
		{"2606:4700:4700:0000:0000:0000:0000:1111", "2606:4700:4700::1111"},
		{"::ffff:8.8.8.8", "::ffff:8.8.8.8"},
	} {
		t.Run(tc.address, func(t *testing.T) {
			for range 2 {
				got, err := service.resolveRoutingPeer(t.Context(), tc.address)
				if err != nil || got != tc.want {
					t.Fatalf("literal validation/canonicalization changed: got=%q want=%q err=%v", got, tc.want, err)
				}
			}
			origin, known := service.routingRuntime.peers[tc.address]
			if !known || origin.address != tc.want || !origin.until.Equal(now.Add(5*time.Minute)) {
				t.Fatal("literal origin or its existing cache lifetime changed", origin)
			}
			if calls.Load() != 0 || !service.routingPeerRefreshAt().IsZero() {
				t.Fatal("literal destination created DNS work or a compile deadline")
			}
		})
	}
}

func TestRoutingLiteralIPsStillRejectNonPublicAddresses(t *testing.T) {
	service, calls := routingLiteralTestService(t)
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "192.0.2.1", "224.0.0.1", "::1", "fc00::1", "fe80::1%eth0", "2001:db8::1", "::ffff:127.0.0.1"} {
		t.Run(address, func(t *testing.T) {
			got, err := service.resolveRoutingPeer(t.Context(), address)
			if !errors.Is(err, ErrInvalid) || got != "" {
				t.Fatalf("non-public literal accepted: got=%q err=%v", got, err)
			}
		})
	}
	if calls.Load() != 0 || len(service.routingRuntime.peers) != 0 {
		t.Fatal("rejected literal entered DNS resolution or the cache")
	}
}

func TestRoutingLiteralIPsKeepAuthenticatedDDNSPrecedence(t *testing.T) {
	service, calls := routingLiteralTestService(t)
	service.routingRuntime.ddns = map[string]string{ddnsOriginKey("node", "8.8.8.8"): "9.9.9.9"}
	if got, err := service.resolveRoutingPeer(t.Context(), "8.8.8.8", "node"); err != nil || got != "9.9.9.9" {
		t.Fatal("literal bypassed the node's authenticated DDNS origin", got, err)
	}
	if got, err := service.resolveRoutingPeer(t.Context(), "8.8.8.8", "other"); err != nil || got != "8.8.8.8" {
		t.Fatal("DDNS origin escaped its node scope", got, err)
	}
	if calls.Load() != 0 || !service.routingPeerRefreshAt().IsZero() {
		t.Fatal("literal/DDNS resolution created DNS work or a refresh deadline")
	}
}

func TestRoutingHostnameCacheStillExpiresAndRejectsFailedRefresh(t *testing.T) {
	service, calls := routingLiteralTestService(t)
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	service.now = func() time.Time { return now }
	host := "routing-peer.example.invalid."
	previous := routingPeer{address: "9.9.9.9", until: now.Add(5 * time.Minute)}
	service.routingRuntime.peers[host] = previous
	if got, err := service.resolveRoutingPeer(t.Context(), host); err != nil || got != previous.address || calls.Load() != 0 {
		t.Fatal("unexpired hostname stopped reusing its DNS result", got, err, calls.Load())
	}
	if !service.routingPeerRefreshAt().Equal(previous.until) {
		t.Fatal("hostname lost its five-minute refresh deadline")
	}
	now = previous.until
	if got, err := service.resolveRoutingPeer(t.Context(), host); !errors.Is(err, ErrInvalid) || got != "" || calls.Load() == 0 {
		t.Fatal("expired hostname reused stale DNS after refresh failure", got, err, calls.Load())
	}
	if service.routingRuntime.peers[host] != previous || !service.routingPeerRefreshAt().IsZero() {
		t.Fatal("failed hostname refresh extended or replaced the old DNS result")
	}
	failedCalls := calls.Load()
	if _, err := service.resolveRoutingPeer(t.Context(), host); !errors.Is(err, ErrInvalid) || calls.Load() <= failedCalls {
		t.Fatal("failed hostname refresh was retained instead of retrying", err, calls.Load())
	}
}

package xnode

import (
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestRoutingLiteralOriginRemainsAvailableForDDNS(t *testing.T) {
	service, calls := routingLiteralTestService(t)
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	service.now = func() time.Time { return now }
	job := wire.DDNSJob{ID: "wan", Provider: "external", Zone: "8.8.8.8", Name: "@", IPv4: "9.9.9.9"}
	if err := job.Validate(); err != nil {
		t.Fatal("numeric DNS owner fixture is invalid", err)
	}
	if _, err := service.resolveRoutingPeer(t.Context(), "8.8.8.8", "node"); err != nil {
		t.Fatal(err)
	}
	if prior := service.previousDDNSOrigin("node", job); len(prior) != 1 || prior[0] != "8.8.8.8" {
		t.Fatal("literal destination lost prior-origin protection", prior)
	}
	previous := service.routingRuntime.peers["8.8.8.8"]
	now = previous.until
	service.routingRuntime.peers["8.8.8.8"] = routingPeer{address: "9.9.9.9", until: previous.until}
	if got, err := service.resolveRoutingPeer(t.Context(), "8.8.8.8", "node"); err != nil || got != "8.8.8.8" {
		t.Fatal("expired literal origin bypassed address validation", got, err)
	}
	if !service.routingRuntime.peers["8.8.8.8"].until.Equal(now.Add(5 * time.Minute)) {
		t.Fatal("literal origin cache no longer renews on use")
	}
	if calls.Load() != 0 || !service.routingPeerRefreshAt().IsZero() {
		t.Fatal("retained literal origin created DNS work or a refresh deadline")
	}
}

func TestRoutingMixedCacheUsesOnlyLiveHostnameDeadlines(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil)
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	service.now = func() time.Time { return now }
	first, second := now.Add(2*time.Minute), now.Add(3*time.Minute)
	service.routingRuntime.peers = map[string]routingPeer{
		"8.8.8.8":              {address: "8.8.8.8", until: now.Add(time.Second)},
		"2606:4700:4700::1111": {address: "2606:4700:4700::1111", until: now.Add(10 * time.Minute)},
		"first.example":        {address: "9.9.9.9", until: first},
		"second.example":       {address: "1.1.1.1", until: second},
	}
	if !service.routingPeerRefreshAt().Equal(first) {
		t.Fatal("literal deadline displaced the earliest hostname deadline")
	}
	now = first
	if !service.routingPeerRefreshAt().Equal(second) {
		t.Fatal("expired hostname displaced the next live hostname deadline")
	}
	now = second
	if !service.routingPeerRefreshAt().IsZero() {
		t.Fatal("literal origin retained a DNS deadline after all hostname results expired")
	}
}

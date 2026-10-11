package iplookup

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestLiveSubscribersShareWorkAndCancelIndependently(t *testing.T) {
	s, _, hosts := integratedLiveService(t, false)
	original := s.client.Transport
	release := make(chan struct{})
	s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "internetdb.shodan.io" {
			select {
			case <-release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		}
		return original.RoundTrip(r)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { _, err := s.Details(ctx, "first", "1.1.1.1"); first <- err }()
	go func() { _, err := s.Details(context.Background(), "second", "1.1.1.1"); second <- err }()
	deadline := time.Now().Add(time.Second)
	joined := false
	for time.Now().Before(deadline) {
		s.mu.Lock()
		for _, f := range s.flights {
			joined = f.waiters == 2
		}
		s.mu.Unlock()
		if joined {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !joined {
		close(release)
		t.Fatal("lookups did not share a flight")
	}
	cancel()
	if err := <-first; err == nil {
		close(release)
		t.Fatal("cancelled subscriber completed")
	}
	close(release)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if hosts.Load() != 1 {
		t.Fatal("shared lookup contacted host source twice")
	}
}

func TestLiveClassificationUsesDatasetDespiteFalsePositiveASNFlags(t *testing.T) {
	s := liveTestService(t, "caida", func(*http.Request) (int, string) {
		return 200, `{"data":{"dataset":{"date":"2026-10-01","clique":["174","7018"]},"asns":{"edges":[{"node":{"asn":"174","asnName":"Cogent","cliqueMember":true}},{"node":{"asn":"6939","asnName":"Hurricane","cliqueMember":true}},{"node":{"asn":"13335","asnName":"Cloudflare","cliqueMember":true}},{"node":{"asn":"5650","asnName":"Frontier","cliqueMember":true}}]}},"errors":[]}`
	})
	graph := BGPTopology{Nodes: []BGPNode{{ASN: 174}, {ASN: 6939}, {ASN: 13335}, {ASN: 5650}, {ASN: 7018}}}
	s.classify(context.Background(), &graph)
	for _, node := range graph.Nodes {
		want := node.ASN == 174 || node.ASN == 7018
		if node.Tier1 == nil || *node.Tier1 != want {
			t.Fatalf("AS%d: false-positive clique recognition", node.ASN)
		}
	}
	if graph.ClassificationMeta.ObservedAt != "2026-10-01" || graph.ClassificationMeta.Status != "success" {
		t.Fatal("lost inference provenance")
	}
}

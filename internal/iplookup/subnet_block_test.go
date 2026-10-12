package iplookup

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestReportedBlockRunsWithSkippedSubnetAndIsReused(t *testing.T) {
	var calls atomic.Int32
	live := liveTestService(t, "abuseipdb", func(r *http.Request) (int, string) {
		if r.URL.Path != "/api/v2/check-block" {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		calls.Add(1)
		return 200, `{"data":{"networkAddress":"8.8.8.0","netmask":"255.255.255.0","reportedAddress":[]}}`
	})
	c := Config{Providers: []ProviderConfig{{ID: "abuseipdb", Enabled: true}}}
	details := live.livePipeline(context.Background(), "8.8.8.8", c, func(LiveEvent) {})
	r := Report{IP: "8.8.8.8", LiveDetails: &details, Subnet: &SubnetResult{Status: "skipped", Reason: "ip_not_passed"}}
	s := NewEnhancedService(nil, nil, live)
	block := s.reportBlock(context.Background(), &r, c)
	if block.Status != "success" || calls.Load() != 1 {
		t.Fatalf("block gated or replayed: %+v calls=%d", block, calls.Load())
	}
}

func TestReportedBlockCanBeDeferredUntilRefusalsAreKnown(t *testing.T) {
	var calls atomic.Int32
	live := liveTestService(t, "abuseipdb", func(*http.Request) (int, string) {
		calls.Add(1)
		return 200, `{"data":{"networkAddress":"8.8.8.0","netmask":"255.255.255.0","reportedAddress":[]}}`
	})
	c := Config{Providers: []ProviderConfig{{ID: "abuseipdb", Enabled: true}}}
	ctx := context.WithValue(context.Background(), deferBlockKey{}, true)
	block := live.block(ctx, "8.8.8.8", c)
	if block.Status != "skipped" || calls.Load() != 0 || block.AddressCapacity != "256" || liveTaskStatus("block", block).Status != "skipped" {
		t.Fatal("deferred block executed or lost prefix capacity")
	}
	block = live.block(context.Background(), "8.8.8.8", c)
	if block.Status != "success" || calls.Load() != 1 {
		t.Fatal("passing checks could not resume the block request")
	}
}

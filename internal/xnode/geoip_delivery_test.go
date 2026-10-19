package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type geoIPQueueFixtureRepository struct {
	Repository
	calls int
}

func (r *geoIPQueueFixtureRepository) QueueXNodeGeoIP(context.Context, string) error {
	r.calls++
	return nil
}

func TestCountryChunkBoundariesAndFinalMarker(t *testing.T) {
	prefixes := make([]string, wire.GeoIPPrefixesPerChunk+1)
	for i := range prefixes {
		prefixes[i] = "8.8.8.0/24"
	}
	policy := &wire.SourceGeoIPPolicy{Generation: "generation", Prefixes: prefixes}
	first, ok := countryChunk(policy, 0)
	if !ok || first.Index != 0 || first.Final || len(first.Prefixes) != wire.GeoIPPrefixesPerChunk {
		t.Fatalf("first country chunk = %#v, %v", first, ok)
	}
	last, ok := countryChunk(policy, 1)
	if !ok || last.Index != 1 || !last.Final || len(last.Prefixes) != 1 {
		t.Fatalf("last country chunk = %#v, %v", last, ok)
	}
	if _, ok = countryChunk(policy, 2); ok {
		t.Fatal("country chunk after final was produced")
	}
}

func TestCountryStateSyncQueuesStreamingOnceAndKeepsLegacyInline(t *testing.T) {
	repo := &geoIPQueueFixtureRepository{}
	now := time.Unix(100, 0)
	service := &Service{repo: repo, now: func() time.Time { return now }}
	master := &Master{service: service}
	peer := &agentPeer{nodeID: "node"}
	policy := &wire.SourceGeoIPPolicy{Generation: "country-v1", Prefixes: []string{"8.8.8.0/24"}}
	config := wire.Config{Firewall: wire.FirewallPolicy{Enabled: true, SourceGeoIP: policy}}

	inline, ready, err := master.prepareCountryForPeer(context.Background(), peer, config, []string{"source_geoip_firewall_v1"})
	if err != nil || !ready || inline.Firewall.SourceGeoIP.Reference || len(inline.Firewall.SourceGeoIP.Prefixes) != 1 || repo.calls != 0 {
		t.Fatalf("legacy country preparation = %#v, ready=%t calls=%d err=%v", inline.Firewall.SourceGeoIP, ready, repo.calls, err)
	}

	streamed, ready, err := master.prepareCountryForPeer(context.Background(), peer, config, []string{"source_geoip_firewall_v1", "source_geoip_stream_v1"})
	if err != nil || ready || streamed.Firewall.SourceGeoIP.Reference || repo.calls != 1 {
		t.Fatalf("unprepared stream = %#v, ready=%t calls=%d err=%v", streamed.Firewall.SourceGeoIP, ready, repo.calls, err)
	}
	_, ready, err = master.prepareCountryForPeer(context.Background(), peer, config, []string{"source_geoip_firewall_v1", "source_geoip_stream_v1"})
	if err != nil || ready || repo.calls != 1 {
		t.Fatalf("duplicate queue intent = ready %t, calls %d, err %v", ready, repo.calls, err)
	}

	peer.geoIPStateMu.Lock()
	peer.geoIPGeneration = policy.Generation
	peer.geoIPStateMu.Unlock()
	reference, ready, err := master.prepareCountryForPeer(context.Background(), peer, config, []string{"source_geoip_firewall_v1", "source_geoip_stream_v1"})
	if err != nil || !ready || !reference.Firewall.SourceGeoIP.Reference || len(reference.Firewall.SourceGeoIP.Prefixes) != 0 || len(policy.Prefixes) != 1 {
		t.Fatalf("staged country reference = %#v, ready=%t err=%v", reference.Firewall.SourceGeoIP, ready, err)
	}
}

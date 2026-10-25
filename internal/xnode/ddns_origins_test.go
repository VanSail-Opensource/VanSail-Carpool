package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type ddnsOriginTestRepository struct {
	Repository
	hosts []Endpoint
}

func (r ddnsOriginTestRepository) XNodeEndpoints(context.Context) ([]Endpoint, error) {
	return r.hosts, nil
}

func TestExternalDDNSOriginsRetainHostnameAndInvalidateOnlyScopedRouting(t *testing.T) {
	ctx := context.Background()
	repo := ddnsOriginTestRepository{hosts: []Endpoint{{ID: "host", NodeID: "node", Address: "node.example.com"}}}
	s := NewService(repo, nil, nil, nil, nil)
	job := wire.DDNSJob{ID: "wan", Provider: "cloudflare", Zone: "example.com", Name: "node.example.com", Generation: "current"}
	jobs := map[string]map[string]wire.DDNSJob{"node": {"wan": job}}
	state := DDNSChangeState{NodeID: "node", Phase: "staging", Change: wire.DDNSChange{JobID: "wan", Generation: "current", IPv4: "9.9.9.9"}}
	s.routingRuntime.peers[job.Name] = routingPeer{address: "8.8.8.8", until: time.Now().Add(time.Hour)}
	if old := s.previousDDNSOrigin("node", job); len(old) != 1 || old[0] != "8.8.8.8" {
		t.Fatal("old cached route address was not retained", old)
	}
	if err := s.publishDDNSOrigins(ctx, []DDNSChangeState{state}, jobs); err != nil || s.ddnsRoutingPeer("node", job.Name) != "" {
		t.Fatal("origin published before staging completed", err)
	}
	state.Phase = "routes"
	if err := s.publishDDNSOrigins(ctx, []DDNSChangeState{state}, jobs); err != nil {
		t.Fatal(err)
	}
	version := s.localCompileRevision.Load()
	if version == 0 || s.ddnsRoutingPeer("node", "NODE.EXAMPLE.COM.") != "9.9.9.9" || s.ddnsRoutingPeer("other-node", job.Name) != "" {
		t.Fatal("wrong origin scope or invalidation")
	}
	if address, err := s.resolveRoutingPeer(ctx, job.Name, "node"); err != nil || address != "9.9.9.9" {
		t.Fatal("stale recursive DNS bypassed authenticated origin", address, err)
	}
	if repo.hosts[0].Address != "node.example.com" {
		t.Fatal("TLS/SNI hostname overwritten")
	}
	if err := s.publishDDNSOrigins(ctx, []DDNSChangeState{state}, jobs); err != nil || s.localCompileRevision.Load() != version {
		t.Fatal("unchanged origin repeatedly invalidated compile cache", err)
	}
	state.Change.Generation = "obsolete"
	if err := s.publishDDNSOrigins(ctx, []DDNSChangeState{state}, jobs); err != nil || s.ddnsRoutingPeer("node", job.Name) != "" {
		t.Fatal("stale configuration retained origin override", err)
	}
	job.Name = "outside.invalid"
	if ddnsHostname(job) != "" {
		t.Fatal("external job outside its zone inferred ownership")
	}
	job.Name = "@"
	if ddnsHostname(job) != "example.com" {
		t.Fatal("zone apex differed from agent semantics")
	}
}

package xnode

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type topologyRepository struct {
	Repository
	version, nodes, groups atomic.Int64
	beforeNodes            func()
	name                   string
}

func (r *topologyRepository) XNodeCompileVersion(ctx context.Context, _ time.Time) (int64, time.Time, error) {
	return r.version.Load(), time.Time{}, ctx.Err()
}
func (r *topologyRepository) XNodeNodes(context.Context) ([]Node, error) {
	r.nodes.Add(1)
	if r.beforeNodes != nil {
		r.beforeNodes()
	}
	return []Node{{ID: "node", Name: r.name, SettingsCipher: "secret", TokenHash: []byte("secret"), Enabled: true}}, nil
}
func (r *topologyRepository) XNodeGroups(context.Context) ([]Group, error) {
	r.groups.Add(1)
	return []Group{{ID: "group", Name: fmt.Sprint(r.version.Load()), Bindings: []GroupBinding{{NodeID: "node", InboundTag: "paid"}}}}, nil
}

func TestCatalogTopologyRevisionIsolationAndBounds(t *testing.T) {
	repo := &topologyRepository{name: "Tokyo"}
	s := NewService(repo, nil, nil, nil, nil)
	ctx := context.Background()
	for range 100 {
		value, err := s.CatalogTopology(ctx)
		if err != nil || value.Nodes[0].Name != "Tokyo" || value.Groups[0].Bindings[0].InboundTag != "paid" {
			t.Fatal(value, err)
		}
		value.Nodes[0].Name = "caller edit"
		value.Groups[0].Bindings[0].InboundTag = "caller edit"
	}
	if repo.nodes.Load() != 1 || repo.groups.Load() != 1 || strings.Contains(string(s.topology.raw), "secret") {
		t.Fatal("unbounded/repeated/private snapshot", repo.nodes.Load(), repo.groups.Load())
	}
	t.Logf("100 catalog topology reads: node queries=%d group queries=%d", repo.nodes.Load(), repo.groups.Load())
	repo.version.Add(1)
	repo.name = "Changed"
	value, err := s.CatalogTopology(ctx)
	if err != nil || value.Nodes[0].Name != "Changed" || repo.nodes.Load() != 2 {
		t.Fatal("revision retained stale metadata", value, err)
	}
	repo.version.Add(1)
	repo.name = strings.Repeat("x", topologyCacheBytes+1)
	if _, err = s.CatalogTopology(ctx); err != nil || len(s.topology.raw) != 0 {
		t.Fatal("oversized metadata retained", err)
	}
}

func TestCatalogTopologyRetriesConcurrentMutationAndCancelsWaiter(t *testing.T) {
	repo := &topologyRepository{}
	repo.beforeNodes = func() {
		if repo.nodes.Load() == 1 {
			repo.version.Add(1)
		}
	}
	s := NewService(repo, nil, nil, nil, nil)
	if _, err := s.CatalogTopology(context.Background()); err != nil || repo.nodes.Load() != 2 {
		t.Fatal("cross-revision candidate published", err, repo.nodes.Load())
	}
	repo.beforeNodes = func() { repo.version.Add(1) }
	repo.version.Add(1)
	if _, err := s.CatalogTopology(context.Background()); !errors.Is(err, ErrConflict) || len(s.topology.raw) != 0 {
		t.Fatal("unstable metadata was retained", err)
	}

	entered, release := make(chan struct{}), make(chan struct{})
	repo.beforeNodes = func() { close(entered); <-release }
	done := make(chan error, 1)
	go func() { _, err := s.CatalogTopology(context.Background()); done <- err }()
	<-entered
	waiting, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := s.CatalogTopology(waiting); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("waiter ignored cancellation", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if repo.nodes.Load() != 6 {
		t.Fatal("waiter loaded duplicate topology", repo.nodes.Load())
	}
}

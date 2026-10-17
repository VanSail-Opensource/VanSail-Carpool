package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type cacheTestRepository struct {
	Repository
	revision int64
	reads    int
	node     Node
}

func (r *cacheTestRepository) XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error) {
	return r.revision, time.Time{}, nil
}
func (r *cacheTestRepository) XNodeNode(context.Context, string) (Node, error) {
	r.reads++
	return r.node, nil
}
func (r *cacheTestRepository) XNodeMigration(context.Context) (MigrationState, error) {
	return MigrationState{Nodes: []MigrationNode{{NodeID: "node", Phase: "legacy"}}}, nil
}

type cacheTestSettings struct{}

func (cacheTestSettings) Optional(context.Context, string) (string, error)     { return "", nil }
func (cacheTestSettings) PutXNodeGlobal(context.Context, string, string) error { return nil }

func TestCompileCacheInvalidatesEpochAndAccessBoundary(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &cacheTestRepository{revision: 1, node: Node{ID: "node"}}
	service := NewService(repo, cacheTestSettings{}, vault, nil, nil)
	repo.node.SettingsCipher, err = service.seal("xnode.node.node", NodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	master := NewMaster(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	peer := &agentPeer{nodeID: "node"}
	ctx := context.Background()
	for range 1000 {
		if _, err = master.cachedConfiguration(ctx, peer, false); err != nil {
			t.Fatal(err)
		}
	}
	if repo.reads != 1 {
		t.Fatal("unchanged state recompiled", repo.reads)
	}
	repo.revision++
	if _, err = master.cachedConfiguration(ctx, peer, false); err != nil {
		t.Fatal(err)
	}
	if repo.reads != 2 {
		t.Fatal("changed state not recompiled")
	}
	peer.compiled.until = time.Now().Add(-time.Second)
	if _, err = master.cachedConfiguration(ctx, peer, false); err != nil {
		t.Fatal(err)
	}
	if repo.reads != 3 {
		t.Fatal("expired access boundary reused")
	}
}

// The hosted job captures both paths with identical synthetic users and nodes.
func BenchmarkXNodeSteadyState(b *testing.B) {
	for _, nodes := range []int{1, 10, 100} {
		config := wire.Config{Xray: json.RawMessage(`{"inbounds":[{"tag":"paid","protocol":"vless","port":443,"settings":{"decryption":"none"}}],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`), Users: []wire.User{}}
		for id := 1; id <= 1000; id++ {
			config.Users = append(config.Users, wire.User{ID: id, UUID: fmt.Sprintf("550e8400-e29b-41d4-a716-%012d", id), InboundTags: []string{"paid"}})
		}
		b.Run(fmt.Sprintf("nodes-%d/before", nodes), func(b *testing.B) {
			for b.Loop() {
				for range nodes {
					if !ValidateCompiled(config).Valid {
						b.Fatal("invalid profile fixture")
					}
					if _, err := json.Marshal(wire.Clone(config)); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
		b.Run(fmt.Sprintf("nodes-%d/after", nodes), func(b *testing.B) {
			master := &Master{globalBlocks: map[int]bool{}}
			for b.Loop() {
				for range nodes {
					_ = master.connectionControls(config)
				}
			}
		})
	}
}

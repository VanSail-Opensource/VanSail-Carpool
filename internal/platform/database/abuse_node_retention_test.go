package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type retentionNodeSource struct {
	nodes []abuse.Node
	err   error
}

func (s retentionNodeSource) AbuseNodes(context.Context) ([]abuse.Node, error) { return s.nodes, s.err }

func TestNodeKeysReconcileDeletedNodesAndPreserveOnFailure(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	source := retentionNodeSource{nodes: []abuse.Node{{UUID: "alive", Name: "Active edge"}, {UUID: "deleted", Name: "Old edge"}}}
	service := abuse.NewService(store, source)
	if _, err := service.SyncNodes(ctx, vault, time.Now()); err != nil {
		t.Fatal(err)
	}
	failing := abuse.NewService(store, retentionNodeSource{err: errors.New("constructed inventory outage")})
	if _, err := failing.SyncNodes(ctx, vault, time.Now()); err == nil {
		t.Fatal("inventory outage ignored")
	}
	before, err := store.NodeCredentials(ctx)
	if err != nil || len(before) != 2 {
		t.Fatalf("credentials erased on outage: %v, %v", before, err)
	}
	source.nodes = source.nodes[:1]
	service = abuse.NewService(store, source)
	if _, err := service.CopyNodeKey(ctx, vault, "deleted"); !errors.Is(err, abuse.ErrNodeMissing) {
		t.Fatalf("deleted copy=%v", err)
	}
	if _, err := service.RotateNodeKey(ctx, vault, "deleted", time.Now()); !errors.Is(err, abuse.ErrNodeMissing) {
		t.Fatalf("deleted rotation=%v", err)
	}
	after, err := service.SyncNodes(ctx, vault, time.Now())
	if err != nil || len(after) != 1 || after[0].UUID != "alive" {
		t.Fatalf("live inventory=%v, %v", after, err)
	}
	if _, err := store.CopyNodeCredential(ctx, "deleted"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted secret retained: %v", err)
	}
	if _, err := service.CopyNodeKey(ctx, vault, "alive"); err != nil {
		t.Fatal(err)
	}
}

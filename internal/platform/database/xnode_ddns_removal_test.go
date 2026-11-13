package database

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestDDNSBindingSaveRejectsConcurrentNodeRemoval(t *testing.T) {
	t.Parallel()
	for _, lifecycle := range []string{"removing_at", "retired_at"} {
		t.Run(lifecycle, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store, identity, _ := xnodeTrafficFixture(t)
			stale, err := store.XNodeNode(ctx, "node")
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SaveXNodeEndpoint(ctx, xnode.Endpoint{ID: "host", NodeID: "node", Name: "Bound host",
				InboundTag: "paid", Address: "203.0.113.7", Port: 443}); err != nil {
				t.Fatal(err)
			}
			binding := xnode.DDNSHostBinding{ConfiguredBy: identity.UserID, IPv4HostIDs: []string{"host"}}
			if err := store.SaveXNodeNodeDDNS(ctx, stale, map[string]xnode.DDNSHostBinding{"wan": binding}); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE xnode_nodes SET enabled=0,`+lifecycle+`=$1 WHERE id='node'`, stamp(time.Now().UTC())); err != nil {
				t.Fatal(err)
			}
			stale.Name, stale.SettingsCipher = "obsolete edit", "obsolete-settings"
			if err := store.SaveXNodeNodeDDNS(ctx, stale, nil); !errors.Is(err, xnode.ErrConflict) {
				t.Fatalf("stale node edit = %v; want lifecycle conflict", err)
			}
			current, err := store.XNodeNode(ctx, "node")
			if err != nil || current.Enabled || current.Name == stale.Name || current.SettingsCipher == stale.SettingsCipher {
				t.Fatalf("removing node changed: %+v, %v", current, err)
			}
			bindings, err := store.XNodeDDNSBindings(ctx, "node")
			if err != nil || len(bindings) != 1 || bindings["wan"].ConfiguredBy != identity.UserID || !slices.Equal(bindings["wan"].IPv4HostIDs, []string{"host"}) {
				t.Fatalf("rejected node save deleted DDNS bindings: %+v, %v", bindings, err)
			}
		})
	}
}

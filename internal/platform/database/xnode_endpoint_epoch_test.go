package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestEndpointMutationInvalidatesRoutingAndDDNSReferences(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "epoch-config", Name: "Config", DocumentCipher: "cipher"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "epoch-node", Name: "Node", ConfigID: "epoch-config", SettingsCipher: "settings", MultiplierFP: 1_000_000}); err != nil {
		t.Fatal(err)
	}
	read := func() int64 {
		t.Helper()
		revision, _, err := s.XNodeCompileVersion(ctx, now)
		if err != nil {
			t.Fatal(err)
		}
		return revision
	}
	host := xnode.Endpoint{ID: "epoch-host", NodeID: "epoch-node", Name: "Host", InboundTag: "paid", Address: "1.1.1.1", Port: 443, Enabled: true}
	previous := read()
	for _, mutation := range []string{"insert", "name", "address", "visibility", "equal"} {
		switch mutation {
		case "name":
			host.Name = "Renamed"
		case "address":
			host.Address = "9.9.9.9"
		case "visibility":
			host.Hidden = true
		}
		if err := s.SaveXNodeEndpoint(ctx, host); err != nil {
			t.Fatal(err)
		}
		current := read()
		if mutation == "equal" {
			if current != previous {
				t.Fatal("equal endpoint invalidated compilation", previous, current)
			}
		} else if current <= previous {
			t.Fatal("endpoint mutation left cached routing/bypass references", mutation, previous, current)
		}
		previous = current
	}
	if err := s.DeleteXNodeEndpoint(ctx, host.ID); err != nil {
		t.Fatal(err)
	}
	if current := read(); current <= previous {
		t.Fatal("last endpoint deletion retained stale bypass references")
	}
}

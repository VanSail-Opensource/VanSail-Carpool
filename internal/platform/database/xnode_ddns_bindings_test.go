package database

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestDDNSBindingOwnershipIsAtomicAndProtectsCredentials(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, 48980)
	if err := s.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "config", Name: "Config", DocumentCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	node := xnode.Node{ID: "node", Name: "Node", ConfigID: "config", SettingsCipher: "discovery-settings", MultiplierFP: 1_000_000}
	other := node
	other.ID = "other"
	for _, item := range []xnode.Node{node, other} {
		if err := s.SaveXNodeNode(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	for _, host := range []xnode.Endpoint{{ID: "v4", NodeID: "node", Name: "IPv4", Address: "203.0.113.7", InboundTag: "paid", Port: 443}, {ID: "v6", NodeID: "node", Name: "IPv6", Address: "2001:db8::7", InboundTag: "paid", Port: 443}, {ID: "foreign", NodeID: "other", Name: "Other", Address: "203.0.113.8", InboundTag: "paid", Port: 443}} {
		if err := s.SaveXNodeEndpoint(ctx, host); err != nil {
			t.Fatal(err)
		}
	}
	token := []byte("existing-private-enrollment-hash")
	if err := s.RotateXNodeToken(ctx, node.ID, token); err != nil {
		t.Fatal(err)
	}
	bindings := map[string]xnode.DDNSHostBinding{"wan": {ConfiguredBy: owner.ID, IPv4HostIDs: []string{"v4"}, IPv6HostIDs: []string{"v6"}}}
	if err := s.SaveXNodeNodeDDNS(ctx, node, bindings); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.XNodeDDNSBindings(ctx, node.ID)
	if err != nil || loaded["wan"].ConfiguredBy != bindings["wan"].ConfiguredBy || !slices.Equal(loaded["wan"].IPv4HostIDs, bindings["wan"].IPv4HostIDs) || !slices.Equal(loaded["wan"].IPv6HostIDs, bindings["wan"].IPv6HostIDs) {
		t.Fatal(loaded, err)
	}
	saved, err := s.XNodeNode(ctx, node.ID)
	if err != nil || !bytes.Equal(saved.TokenHash, token) {
		t.Fatal("binding save changed credentials", err)
	}
	for _, invalid := range []xnode.DDNSHostBinding{{ConfiguredBy: owner.ID, IPv4HostIDs: []string{"v4"}, IPv6HostIDs: []string{"v4"}}, {ConfiguredBy: owner.ID, IPv4HostIDs: []string{"foreign"}}} {
		changed := node
		changed.Name = "Must roll back"
		if err = s.SaveXNodeNodeDDNS(ctx, changed, map[string]xnode.DDNSHostBinding{"wan": invalid}); err == nil {
			t.Fatal("invalid binding committed")
		}
		saved, err = s.XNodeNode(ctx, node.ID)
		if err != nil || saved.Name != node.Name || !bytes.Equal(saved.TokenHash, token) {
			t.Fatal("failed binding partly mutated node", saved, err)
		}
	}
	if err = s.DeleteXNodeEndpoint(ctx, "v4"); err == nil {
		t.Fatal("bound host was deleted")
	}
	if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_endpoints SET node_id='other' WHERE id='v4'`); err == nil {
		t.Fatal("bound host escaped node ownership")
	}
	if _, err = s.DB().ExecContext(ctx, `UPDATE users SET role='user',onboarding_state='intro',created_at=? WHERE id=?`, stamp(time.Now().Add(-72*time.Hour)), owner.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = pruneAbandonedUsersTx(ctx, tx, time.Now().Add(-24*time.Hour), map[string]int64{}); err != nil {
		t.Fatal("maintenance did not protect DDNS configuration owner", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UserByID(ctx, owner.ID); err != nil {
		t.Fatal("DDNS owner removed", err)
	}
}

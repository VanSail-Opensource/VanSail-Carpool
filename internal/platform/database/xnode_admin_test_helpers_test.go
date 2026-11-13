package database

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type xnodeAdminFixture struct {
	store    *Store
	actor    string
	config   xnode.Configuration
	snippet  xnode.Snippet
	node     xnode.Node
	group    xnode.Group
	endpoint xnode.Endpoint
	identity xnode.Identity
	access   xnode.ServiceAccess
	custom   xnode.Customization
}

func newXNodeAdminFixture(t *testing.T) xnodeAdminFixture {
	t.Helper()
	ctx, store := context.Background(), newTestStore(t)
	f := xnodeAdminFixture{store: store, actor: createTestAdmin(t, store, 969001).ID}
	for _, id := range []string{"snippet", "delete-snippet"} {
		if err := store.SaveXNodeSnippet(ctx, xnode.Snippet{ID: id, Name: id, Kind: "outbound", DocumentCipher: "secret-snippet"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"config", "delete-config"} {
		if err := store.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: id, Name: id, DocumentCipher: "secret-config", SnippetIDs: []string{"snippet"}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"node", "delete-node"} {
		node := xnode.Node{ID: id, Name: id, ConfigID: "config", SettingsCipher: "secret-settings", Enabled: id == "node"}
		if err := store.SaveXNodeNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"group", "delete-group"} {
		if err := store.SaveXNodeGroup(ctx, xnode.Group{ID: id, Name: id, Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: "node", InboundTag: "main"}}}); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	if f.config, err = store.XNodeConfiguration(ctx, "config"); err != nil {
		t.Fatal(err)
	}
	if f.snippet, err = store.XNodeSnippet(ctx, "snippet"); err != nil {
		t.Fatal(err)
	}
	if f.node, err = store.XNodeNode(ctx, "node"); err != nil {
		t.Fatal(err)
	}
	f.group = xnode.Group{ID: "group", Name: "after-group", Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: "node", InboundTag: "changed"}}}
	f.endpoint = xnode.Endpoint{ID: "endpoint", NodeID: "node", InboundTag: "main", Name: "endpoint", Address: "example.org", Port: 443, Enabled: true, OverridesCipher: "secret-overrides"}
	if err = store.SaveXNodeEndpoint(ctx, f.endpoint); err != nil {
		t.Fatal(err)
	}
	identity, access := namedCadenceFixture()
	identity.CredentialsCipher, identity.SubscriptionCipher = "secret-credentials", "secret-subscription"
	hash := sha256.Sum256([]byte("old-subscription"))
	identity.SubscriptionHash = hash[:]
	if f.identity, err = store.SaveXNodeNamedService(ctx, identity, access); err != nil {
		t.Fatal(err)
	}
	f.access = access
	f.custom = xnode.Customization{ProxyUserID: f.identity.ProxyUserID, NodeID: "node", OutboundsCipher: "secret-outbounds", RoutingCipher: "secret-routing"}
	if err = store.SaveXNodeCustomization(ctx, f.custom); err != nil {
		t.Fatal(err)
	}
	f.custom.Revision = 1
	return f
}

type xnodeAdminAttempt struct {
	name  string
	write func(string) error
}

func (f xnodeAdminFixture) attempts() []xnodeAdminAttempt {
	ctx, s := context.Background(), f.store
	config, snippet, node, endpoint, identity, custom := f.config, f.snippet, f.node, f.endpoint, f.identity, f.custom
	config.Name, snippet.Name, node.Name, endpoint.Name = "after-config", "after-snippet", "after-node", "after-endpoint"
	identity.CredentialsCipher, custom.RoutingCipher = "secret-new-credentials", "secret-new-routing"
	hash := sha256.Sum256([]byte("new-secret-bearer"))
	access := f.access
	access.LimitBytes, access.Disabled = 2000, true
	return []xnodeAdminAttempt{
		{"configuration save", func(actor string) error { _, err := s.SaveAdminXNodeConfiguration(ctx, actor, config); return err }},
		{"configuration delete", func(actor string) error { return s.DeleteAdminXNodeConfiguration(ctx, actor, "delete-config") }},
		{"snippet save", func(actor string) error { _, err := s.SaveAdminXNodeSnippet(ctx, actor, snippet); return err }},
		{"snippet delete", func(actor string) error { return s.DeleteAdminXNodeSnippet(ctx, actor, "delete-snippet") }},
		{"group save", func(actor string) error { _, err := s.SaveAdminXNodeGroup(ctx, actor, f.group); return err }},
		{"group delete", func(actor string) error {
			version, _, err := s.XNodeCompileVersion(ctx, time.Now())
			if err != nil {
				return err
			}
			return s.DeleteAdminXNodeGroupAtVersion(ctx, actor, "delete-group", version, nil)
		}},
		{"endpoint save", func(actor string) error { _, err := s.SaveAdminXNodeEndpoint(ctx, actor, endpoint); return err }},
		{"endpoint delete", func(actor string) error { return s.DeleteAdminXNodeEndpoint(ctx, actor, "endpoint") }},
		{"node save", func(actor string) error {
			_, err := s.SaveAdminXNodeNodeDDNS(ctx, actor, node, map[string]xnode.DDNSHostBinding{"new-job": {ConfiguredBy: actor}})
			return err
		}},
		{"node delete", func(actor string) error { return s.DeleteAdminXNodeNode(ctx, actor, "delete-node") }},
		{"node token", func(actor string) error { return s.RotateAdminXNodeToken(ctx, actor, "node", hash[:]) }},
		{"named service", func(actor string) error {
			_, err := s.SaveAdminXNodeNamedService(ctx, actor, identity, access)
			return err
		}},
		{"customization", func(actor string) error { _, err := s.SaveAdminXNodeCustomization(ctx, actor, custom); return err }},
		{"subscription", func(actor string) error {
			current, err := s.XNodeIdentity(ctx, f.identity.ProxyUserID)
			if err != nil {
				return err
			}
			_, err = s.RotateAdminXNodeSubscription(ctx, actor, current, hash[:], "secret-new-subscription")
			return err
		}},
	}
}

func xnodeAdminSnapshot(t *testing.T, store *Store) string {
	t.Helper()
	var snapshot strings.Builder
	// Include all changed rows and trigger-maintained versions, excluding the
	// allocation sequence, whose gaps after PostgreSQL rollback are harmless.
	for _, table := range []string{"xnode_configs", "xnode_snippets", "xnode_config_snippets", "xnode_nodes", "xnode_ddns_jobs", "xnode_ddns_hosts", "xnode_groups", "xnode_group_bindings", "xnode_endpoints", "xnode_identities", "xnode_periods", "xnode_customizations", "xnode_compile_revision", "audit_events"} {
		var rows string
		if err := store.DB().QueryRowContext(context.Background(), `SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]'::jsonb)::text FROM `+table+` t`).Scan(&rows); err != nil {
			t.Fatal(table, err)
		}
		snapshot.WriteString(table + ":" + rows + "\n")
	}
	return snapshot.String()
}

package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestStandaloneAuthorizationKeepsQuotaAndGroupPolicies(t *testing.T) {
	for _, tc := range []struct {
		name        string
		limit, used int64
		blocked     bool
	}{
		{"finite", 1000, 100, false},
		{"unlimited", 0, 10000, false},
		{"exhausted", 1000, 1000, true},
		{"disabled identity", 1000, 100, true},
		{"disabled group", 1000, 100, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, identity, _ := standaloneFixture(t, tc.limit, tc.used, "NO_RESET")
			ctx := context.Background()
			if tc.name == "disabled identity" {
				if err := f.store.SetXNodeAccessDisabled(ctx, identity.ProxyUserID, true); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "disabled group" {
				f.group.Enabled = false
				if _, err := f.service.SaveGroup(ctx, f.group); err != nil {
					t.Fatal(err)
				}
			}
			compiled, err := f.service.Compile(ctx, f.node.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.blocked {
				if len(compiled.Users) != 0 {
					t.Fatal("restricted standalone account remains authorized")
				}
				return
			}
			if len(compiled.Users) != 1 || compiled.Users[0].AccountingPeriod != identity.CurrentPeriodID || compiled.Users[0].ConnectionLimit != 0 || compiled.Users[0].SpeedLimitMbps != 0 {
				t.Fatal("standalone account inherited probe limits or lost accounting")
			}
		})
	}
}

func TestStandaloneInventoryKeepsSourceAndNativeCredentialsSeparate(t *testing.T) {
	f, identity, _ := standaloneFixture(t, 1000, 100, "NO_RESET")
	ctx := context.Background()
	if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Native host", Address: "native.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	legacy, err := f.service.SaveNode(ctx, xnode.Node{Name: "Source owned", Enabled: true, Multiplier: 1, ConfigID: f.node.ConfigID}, xnode.NodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: legacy.ID, InboundTag: "paid", Name: "Legacy host", Address: "legacy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	f.group.Bindings = append(f.group.Bindings, xnode.GroupBinding{NodeID: legacy.ID, InboundTag: "paid"})
	if _, err = f.service.SaveGroup(ctx, f.group); err != nil {
		t.Fatal(err)
	}
	if err = f.store.SaveXNodeMigrationNode(ctx, xnode.MigrationNode{NodeID: legacy.ID, LegacyNodeID: legacy.ID, Phase: "legacy"}); err != nil {
		t.Fatal(err)
	}
	f.service.SetLegacyProxies(func(context.Context, xnode.Identity) ([]subscription.Proxy, error) {
		return []subscription.Proxy{{ID: "legacy-host", NodeID: legacy.ID, Address: "legacy.example", Port: 443, Protocol: "shadowsocks", Account: map[string]any{"password": "unchanged-source-key", "method": "chacha20-ietf-poly1305"}}}, nil
	})
	proxies, err := f.service.ServiceProxies(ctx, identity)
	if err != nil || len(proxies) != 2 {
		t.Fatalf("mixed-provider inventory=%d, %v", len(proxies), err)
	}
	for _, proxy := range proxies {
		if proxy.NodeID == legacy.ID && proxy.Account["password"] != "unchanged-source-key" {
			t.Fatal("native credentials replaced the serving source credential")
		}
		if proxy.NodeID == f.node.ID && proxy.Account["id"] != "00000000-0000-4000-8000-000000000376" {
			t.Fatal("native compatible credential changed")
		}
	}
}

func TestStandaloneSubscriptionHonorsHostACLAndExcludesProbeBearers(t *testing.T) {
	f, identity, _ := standaloneFixture(t, 1000, 100, "NO_RESET")
	ctx := context.Background()
	endpoint, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Restricted host", Address: "proxy.example", Port: 443, Enabled: true, Overrides: json.RawMessage(`{"legacyHost":{"internalSquads":{"mode":"ALLOW_ONLY","squads":["foreign"]}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := f.service.ServiceSubscription(ctx, stringID(identity))
	if err != nil || metadata.Active {
		t.Fatalf("standalone subscription bypassed host ACL: %v", err)
	}
	probe, err := f.service.AuthorizeService(ctx, "bounded-probe", endpoint, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	link, err := f.service.RevokeSubscription(ctx, stringID(probe))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.BearerMetadata(ctx, link[strings.LastIndex(link, "/")+1:]); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatalf("probe acquired standalone subscription access: %v", err)
	}
}

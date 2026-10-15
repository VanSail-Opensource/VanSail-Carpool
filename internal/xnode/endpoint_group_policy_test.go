package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestImportedHostACLPreventsSubscriptionAndRoutingTemplateLeakage(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	endpoint := xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Host", Address: "proxy.example", Port: 443, Enabled: true}
	for _, scenario := range []struct {
		name, mode string
		squads     []string
		expected   int
	}{
		{"allow owned", "ALLOW_ONLY", []string{f.group.ID}, 1},
		{"deny foreign allow-only", "ALLOW_ONLY", []string{"foreign"}, 0},
		{"empty allow-only denies all", "ALLOW_ONLY", []string{}, 0},
		{"excluded owned", "EXCLUDE", []string{f.group.ID}, 0},
		{"excluded foreign", "EXCLUDE", []string{"foreign"}, 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			endpoint.Overrides, _ = json.Marshal(map[string]any{"legacyHost": map[string]any{"internalSquads": map[string]any{"mode": scenario.mode, "squads": scenario.squads}}})
			var err error
			endpoint, err = f.service.SaveEndpoint(ctx, endpoint)
			if err != nil {
				t.Fatal(err)
			}
			proxies, err := f.service.MemberProxies(ctx, f.user.ID)
			if err != nil || len(proxies) != scenario.expected {
				t.Fatalf("proxies=%d err=%v", len(proxies), err)
			}
			templates, err := f.service.RoutingTemplates(ctx, f.user.ID)
			if err != nil || len(templates) != scenario.expected {
				t.Fatalf("routing templates=%d err=%v", len(templates), err)
			}
		})
	}
	endpoint.Overrides = json.RawMessage(`{"legacyHost":{"internalSquads":{"mode":"UNKNOWN","squads":[]}}}`)
	if _, err := f.service.SaveEndpoint(ctx, endpoint); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatalf("unknown ACL accepted=%v", err)
	}
}

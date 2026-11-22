package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestInventoryAndMetricsReuseOnlyRequestTopology(t *testing.T) {
	for _, metrics := range []bool{false, true} {
		name := "inventory"
		if metrics {
			name = "metrics"
		}
		t.Run(name, func(t *testing.T) {
			s, r := inventoryProjectionFixture(t)
			if metrics {
				if _, err := NewMaster(s, nil).UserMetrics(context.Background(), "member"); err != nil {
					t.Fatal(err)
				}
			} else {
				proxies, err := s.MemberProxies(context.Background(), "member")
				if err != nil || len(proxies) != 24 {
					t.Fatalf("shared endpoints lost billing principals: count=%d err=%v", len(proxies), err)
				}
				byPrincipal := map[int64]string{}
				for _, proxy := range proxies {
					credential := proxy.Account["id"].(string)
					if previous := byPrincipal[proxy.PrincipalID]; previous != "" && previous != credential {
						t.Fatal("one principal received inconsistent credentials")
					}
					byPrincipal[proxy.PrincipalID] = credential
				}
				if len(byPrincipal) != 3 || byPrincipal[2] == byPrincipal[3] || byPrincipal[3] == byPrincipal[4] || byPrincipal[2] == byPrincipal[4] {
					t.Fatal("shared native input mixed principal credentials", byPrincipal)
				}
				proxies[0].Account["id"] = "changed"
				proxies[0].Stream["wsSettings"].(map[string]any)["path"] = "/changed"
				proxies[0].GroupIDs[0] = "changed"
				if proxies[1].Account["id"] == "changed" || proxies[1].Stream["wsSettings"].(map[string]any)["path"] != "/native" || proxies[1].GroupIDs[0] == "changed" {
					t.Fatal("projected clients share mutable fields")
				}
			}
			if r.groupReads != 1 || r.endpointReads != 1 || r.configReads != 1 || r.snippetReads != 1 || r.nodeReads["node"] != 2 || r.nodeReads["foreign"] != 0 || r.identityScans != 0 {
				t.Fatalf("per-endpoint SQL amplification: groups=%d endpoints=%d config=%d snippets=%d nodes=%v identities=%d", r.groupReads, r.endpointReads, r.configReads, r.snippetReads, r.nodeReads, r.identityScans)
			}
		})
	}
}

func TestInventoryProjectionReloadsAuthorizationAndTopology(t *testing.T) {
	s, r := inventoryProjectionFixture(t)
	ctx := context.Background() // Reusing an HTTP context must not retain a view.
	for _, step := range []struct {
		name   string
		change func()
		count  int
	}{
		{"initial", func() {}, 24},
		{"group disabled", func() { r.groups[0].Enabled = false }, 16},
		{"principal disabled", func() { i := r.identities[3]; i.AccessDisabled = true; r.identities[3] = i }, 8},
		{"hourly disabled", func() { r.hourly[0].Status = "disabled" }, 0},
		{"hourly funding pause", func() { r.hourly[0].Status = "funding_suspended" }, 8},
		{"host ACL", func() { setInventoryOverrides(t, s, r, json.RawMessage(`{"allowedGroups":[]}`)) }, 0},
		{"host ACL restored", func() { setInventoryOverrides(t, s, r, json.RawMessage(`{}`)) }, 8},
		{"node disabled", func() { r.node.Enabled = false }, 0},
		{"node restored", func() { r.node.Enabled = true }, 8},
		{"owner restricted", func() { r.restricted = true }, 0},
	} {
		t.Run(step.name, func(t *testing.T) {
			step.change()
			before := r.accountReads
			proxies, err := s.MemberProxies(ctx, "member")
			if err != nil || len(proxies) != step.count {
				t.Fatalf("stale inventory after %s: count=%d err=%v", step.name, len(proxies), err)
			}
			if !r.restricted && r.accountReads <= before {
				t.Fatal("financial eligibility was reused between reads")
			}
		})
	}
}

func TestInventoryProjectionPreservesReadAndCredentialFailures(t *testing.T) {
	s, r := inventoryProjectionFixture(t)
	ctx := context.Background()
	r.configurationErr = errors.New("configuration read unavailable")
	if _, err := s.MemberProxies(ctx, "member"); !errors.Is(err, r.configurationErr) {
		t.Fatal("configuration storage failure became an empty inventory", err)
	}
	r.configurationErr = nil
	if proxies, err := s.MemberProxies(ctx, "member"); err != nil || len(proxies) != 24 {
		t.Fatal("transient read failure was retained", len(proxies), err)
	}
	identity := r.identities[2]
	identity.CredentialsCipher = "invalid"
	r.identities[2] = identity
	if _, err := s.MemberProxies(ctx, "member"); err == nil {
		t.Fatal("credential corruption was hidden by a projection cache")
	}
}

func TestInventoryProjectionReloadsChangedConfiguration(t *testing.T) {
	s, r := inventoryProjectionFixture(t)
	ctx := context.Background()
	for _, path := range []string{"/initial", "/updated"} {
		raw, err := json.Marshal(map[string]any{"inbounds": []any{map[string]any{
			"tag": "paid", "protocol": "vless", "settings": map[string]any{"decryption": "none"},
			"streamSettings": map[string]any{"network": "ws", "wsSettings": map[string]any{"path": path}},
		}}})
		if err != nil {
			t.Fatal(err)
		}
		r.configuration.DocumentCipher, err = s.vault.Encrypt("xnode.config.configuration", string(raw))
		if err != nil {
			t.Fatal(err)
		}
		proxies, err := s.MemberProxies(ctx, "member")
		if err != nil || len(proxies) != 24 {
			t.Fatal(len(proxies), err)
		}
		for _, proxy := range proxies {
			if proxy.Stream["wsSettings"].(map[string]any)["path"] != path {
				t.Fatal("previous request's native configuration was retained")
			}
		}
	}
}

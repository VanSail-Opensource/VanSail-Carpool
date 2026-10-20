package xnode_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestManagedRoutingSupportsProductionContextsWithoutWeakeningCycles(t *testing.T) {
	for _, test := range []struct {
		name, strategy         string
		sniff, fallback, cycle bool
		valid                  bool
	}{
		{"sniffed one-way", "AsIs", true, false, false, true},
		{"DNS one-way", "IPOnDemand", false, false, false, true},
		{"production one-way", "IPOnDemand", true, true, false, true},
		{"fixed disjoint cycle", "AsIs", false, false, true, true},
		{"sniffed disjoint cycle", "AsIs", true, false, true, false},
		{"DNS disjoint cycle", "IPOnDemand", false, false, true, false},
		{"untracked disjoint cycle", "AsIs", false, true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := controlFixture(t, true)
			ctx := context.Background()
			profile, err := f.service.ResolveConfiguration(ctx, f.node.ConfigID)
			if err != nil {
				t.Fatal(err)
			}
			var native map[string]any
			if err = json.Unmarshal(profile.Document, &native); err != nil {
				t.Fatal(err)
			}
			// Public VLESS endpoints require a supported encrypted transport.
			// Match the existing deployment fixture so inventory validation
			// reaches the routing proof for both acceptance and rejection cases.
			key, err := ecdh.X25519().GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			inbound := native["inbounds"].([]any)[0].(map[string]any)
			inbound["streamSettings"] = map[string]any{"network": "tcp", "security": "reality", "realitySettings": map[string]any{"dest": "www.example.com:443", "serverNames": []string{"www.example.com"}, "privateKey": base64.RawURLEncoding.EncodeToString(key.Bytes()), "shortIds": []string{"0123456789abcdef"}}}
			routing := native["routing"].(map[string]any)
			routing["domainStrategy"] = test.strategy
			if test.sniff {
				inbound["sniffing"] = map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}}
			}
			if test.fallback {
				proxy := map[string]any{"tag": "admin-exit", "protocol": "socks", "settings": map[string]any{"servers": []any{map[string]any{"address": "127.0.0.1", "port": 1080}}}}
				native["outbounds"] = append([]any{proxy}, native["outbounds"].([]any)...)
				routing["rules"] = append(routing["rules"].([]any), map[string]any{"type": "field", "network": "tcp,udp", "outboundTag": "admin-exit"})
			}
			profile.Document, err = json.Marshal(native)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.service.SaveConfiguration(ctx, profile); err != nil {
				t.Fatal(err)
			}
			peer, err := f.service.SaveNode(ctx, xnode.Node{Name: "Production-style peer", Enabled: true, Multiplier: 1, ConfigID: profile.ID}, xnode.NodeSettings{})
			if err != nil {
				t.Fatal(err)
			}
			f.group.Bindings = append(f.group.Bindings, xnode.GroupBinding{NodeID: peer.ID, InboundTag: "paid"})
			if _, err = f.service.SaveGroup(ctx, f.group); err != nil {
				t.Fatal(err)
			}
			endpoints := []xnode.Endpoint{}
			for index, node := range []xnode.Node{f.node, peer} {
				endpoint, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{Name: node.Name, NodeID: node.ID, InboundTag: "paid", Address: []string{"1.1.1.1", "8.8.8.8"}[index], Port: 443, Enabled: true})
				if err != nil {
					t.Fatal(err)
				}
				endpoints = append(endpoints, endpoint)
			}
			proxies, err := f.service.MemberProxies(ctx, f.user.ID)
			if err != nil {
				var diagnostic *xnode.CompilationError
				if errors.As(err, &diagnostic) {
					t.Fatalf("fixture endpoint inventory: %v: %s", err, diagnostic.ValidationDetail())
				}
				t.Fatal(err)
			}
			if len(proxies) != 2 {
				t.Fatal("fixture did not publish both eligible endpoints", len(proxies))
			}
			templates := []xnode.RoutingTemplate{{ID: "google", Name: "Google", Match: routingmatch.Match{Domains: []string{"domain:google.com"}}}, {ID: "ai", Name: "AI", Match: routingmatch.Match{Domains: []string{"domain:openai.com"}}}}
			if _, err = f.service.SaveRoutingTemplates(ctx, f.user.ID, xnode.RoutingTemplateConfig{Templates: templates}); err != nil {
				t.Fatal(err)
			}
			choices := []xnode.RoutingChoice{{ID: "ab", InboundNodeID: f.node.ID, OutboundNodeID: peer.ID, InboundEndpointID: endpoints[0].ID, OutboundEndpointID: endpoints[1].ID, TemplateID: "google"}}
			if test.cycle {
				choices = append(choices, xnode.RoutingChoice{ID: "ba", InboundNodeID: peer.ID, OutboundNodeID: f.node.ID, InboundEndpointID: endpoints[1].ID, OutboundEndpointID: endpoints[0].ID, TemplateID: "ai"})
			}
			_, trace, err := f.service.PreviewOwnedRouting(ctx, f.user.ID, choices, f.node.ID, routingmatch.Destination{Domain: "www.google.com"}, endpoints[0].ID)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v: %v", test.valid, err)
			}
			if !test.valid {
				if !errors.Is(err, xnode.ErrInvalid) {
					t.Fatal("unsafe context did not fail closed", err)
				}
				var diagnostic *routingmatch.Diagnostic
				if test.strategy == "IPOnDemand" {
					if !strings.Contains(err.Error(), "requires domainStrategy AsIs") {
						t.Fatal("DNS cycle failed before its context guard", err)
					}
				} else if !errors.As(err, &diagnostic) || diagnostic.Code != "routing_context_unproven" {
					t.Fatal("unsafe cycle failed before its context guard", err)
				}
				return
			}
			if trace == nil || !slices.Equal(trace.Nodes, []string{f.node.ID, peer.ID}) {
				t.Fatal("preview lost the selected hop", trace)
			}
			// A genuinely overlapping return must still fail even with a safe
			// context; structural acceptance never bypasses matcher validation.
			if test.cycle {
				choices[1].TemplateID = "google"
				_, _, err = f.service.PreviewOwnedRouting(ctx, f.user.ID, choices, "", routingmatch.Destination{})
				var diagnostic *routingmatch.Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Code != "routing_loop" {
					t.Fatal("overlapping cycle admitted", err)
				}
			}
		})
	}
}

package xnode_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestChangedInboundKeepsCompatibleSubscriptionPeers(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	profile, err := f.service.ResolveConfiguration(ctx, f.node.ConfigID)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if json.Unmarshal(profile.Document, &native) != nil {
		t.Fatal("fixture document")
	}
	inbound := native["inbounds"].([]any)[0].(map[string]any)
	inbound["streamSettings"] = map[string]any{"network": "xhttp", "xhttpSettings": map[string]any{"mode": "packet-up", "path": "/xhttp"}}
	native["inbounds"] = append(native["inbounds"].([]any), map[string]any{"tag": "peer", "protocol": "vless", "port": 8443, "settings": map[string]any{"decryption": "none"}, "streamSettings": map[string]any{"network": "ws", "wsSettings": map[string]any{"path": "/peer"}}})
	profile.Document, _ = json.Marshal(native)
	if profile, err = f.service.SaveConfiguration(ctx, profile); err != nil {
		t.Fatal(err)
	}
	group := f.group
	group.Bindings = append(group.Bindings, xnode.GroupBinding{NodeID: f.node.ID, InboundTag: "peer"})
	if _, err = f.service.SaveGroup(ctx, group); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.SaveEndpoint(ctx, xnode.Endpoint{Name: "Mode dependent", NodeID: f.node.ID, InboundTag: "paid", Address: "dependent.example", Port: 443, Enabled: true,
		Overrides: json.RawMessage(`{"clientOptions":{"xhttpUplinkHTTPMethod":"GET"}}`)}); err != nil {
		t.Fatal(err)
	}
	peer, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{Name: "Compatible peer", NodeID: f.node.ID, InboundTag: "peer", Address: "peer.example", Port: 8443, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	inbound["streamSettings"].(map[string]any)["xhttpSettings"].(map[string]any)["mode"] = "stream-up"
	profile.Document, _ = json.Marshal(native)
	if _, err = f.service.SaveConfiguration(ctx, profile); err != nil {
		t.Fatal(err)
	}
	proxies, err := f.service.MemberProxies(ctx, f.user.ID)
	if err != nil {
		t.Fatal("one changed endpoint removed the member inventory", err)
	}
	for _, format := range []string{"native", "base64", "xray", "sing-box", "clash", "stash"} {
		selected, _, err := subscription.CompatibleProxies(format, proxies)
		if err != nil {
			t.Fatal(format, err)
		}
		found := false
		for _, proxy := range selected {
			found = found || proxy.ID == peer.ID
			if format == "xray" {
				out, err := proxy.Outbound()
				raw, _ := json.Marshal(map[string]any{"inbounds": []any{}, "outbounds": []any{out}})
				validation := xnode.ValidateClientConfigurationDocument(raw)
				if err != nil || !validation.Valid {
					t.Fatal("stale dependency produced an invalid native client", err, validation.Detail)
				}
			}
		}
		if !found {
			t.Fatal("compatible peer was lost", format)
		}
	}
}

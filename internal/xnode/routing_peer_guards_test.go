package xnode

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"testing"
)

func TestRoutingPeerMappersCannotReplaceAuthenticationOrHop(t *testing.T) {
	base := subscription.Proxy{ID: "peer", Protocol: "vless", Address: "1.1.1.1", Port: 443, Account: map[string]any{"id": "private-owner-credential", "encryption": "none"}, Stream: map[string]any{"network": "tcp", "security": "tls"}}
	for _, field := range []string{"settings.vnext[0].address", "settings.vnext[0].users[0].id", "protocol", "streamSettings.sockopt.dialerProxy"} {
		t.Run(field, func(t *testing.T) {
			p := base
			p.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{"xrayJson": []any{map[string]any{"op": "set", "to": field, "value": "unmodelled-hop"}}}}}
			if _, err := guardedRoutingOutbound(p); err == nil {
				t.Fatal("mapper escaped private managed peer binding")
			}
			if base.Account["id"] != "private-owner-credential" {
				t.Fatal("mapper mutated shared private account")
			}
		})
	}
	p := base
	p.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{"xrayJson": []any{map[string]any{"op": "set", "to": "streamSettings.tlsSettings.serverName", "value": "peer.example"}}}}}
	if _, err := guardedRoutingOutbound(p); err != nil {
		t.Fatal("safe client transport field rejected", err)
	}
}
func TestRoutingContextRejectsDestinationRewriteAndUnstableProtocol(t *testing.T) {
	root := map[string]any{"outbounds": []any{map[string]any{"tag": "direct", "protocol": "freedom"}}, "routing": map[string]any{"rules": []any{map[string]any{"UserRouting": map[string]any{}}}}, "inbounds": []any{map[string]any{"tag": "paid", "sniffing": map[string]any{"enabled": true, "destOverride": []any{"tls"}, "routeOnly": true}}}}
	if _, err := routingServerContext(root, []string{"paid"}, false, false); err == nil {
		t.Fatal("per-hop destination rewrite admitted as fixed metadata")
	}
	root["inbounds"] = []any{map[string]any{"tag": "paid"}}
	if _, err := routingServerContext(root, []string{"paid"}, true, false); err != nil {
		t.Fatal("protocol-only detector changed domain routing", err)
	}
	if _, err := routingServerContext(root, []string{"paid"}, true, true); err == nil {
		t.Fatal("variable protocol detection admitted as fixed metadata")
	}
}

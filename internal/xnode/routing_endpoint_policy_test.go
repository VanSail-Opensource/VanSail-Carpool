package xnode

import (
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func routingPolicyProxy() subscription.Proxy {
	return subscription.Proxy{
		ID: "peer", Protocol: "vless", Address: "1.1.1.1", Port: 443,
		Account: map[string]any{"id": "c1d50c69-2f17-41ec-a23c-b30eb375639f", "encryption": "none"},
		Stream:  map[string]any{"network": "tcp", "security": "tls", "tlsSettings": map[string]any{"serverName": "peer.example"}},
	}
}

func routingPolicyNativeGuard(out map[string]any) error {
	raw, err := json.Marshal(map[string]any{"outbounds": []any{out}})
	if err != nil {
		return err
	}
	return wire.ValidateMemberOutbounds(raw, []string{out["tag"].(string)})
}

func TestManagedRoutingOmitsOnlyNeutralClientMPTCPFlag(t *testing.T) {
	fingerprint := "firefox"
	for _, test := range []struct {
		name    string
		options *subscription.EndpointOptions
		socket  map[string]any
		xhttp   bool
		mux     map[string]any
	}{
		{"legacy explicit false", nil, map[string]any{"tcpMptcp": false}, false, nil},
		{"default typed endpoint", &subscription.EndpointOptions{}, nil, false, nil},
		{"typed security option", &subscription.EndpointOptions{Fingerprint: &fingerprint}, nil, false, nil},
		{"disabled client mux", &subscription.EndpointOptions{}, nil, false, map[string]any{"enabled": false}},
		{"XHTTP native XMUX", &subscription.EndpointOptions{}, nil, true, map[string]any{"enabled": true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := routingPolicyProxy()
			p.EndpointTuning = test.options
			p.Mux = test.mux
			if test.xhttp {
				p.Stream["network"] = "xhttp"
				p.Stream["xhttpSettings"] = map[string]any{"path": "/session", "mode": "packet-up", "extra": map[string]any{"xmux": map[string]any{"maxConcurrency": 16}}}
			}
			if test.socket != nil {
				p.Stream["sockopt"] = test.socket
			}
			before, err := json.Marshal(map[string]any{"stream": p.Stream, "mux": p.Mux})
			if err != nil {
				t.Fatal(err)
			}
			out, err := guardedRoutingOutbound(p)
			if err != nil {
				t.Fatal("valid endpoint cannot become a managed peer", err)
			}
			stream := out["streamSettings"].(map[string]any)
			if _, exists := stream["sockopt"]; exists {
				t.Fatal("neutral export policy became a managed socket override", stream)
			}
			if err = routingPolicyNativeGuard(out); err != nil {
				t.Fatal("managed peer violates pinned native member guard", err)
			}
			if test.xhttp && out["mux"] != nil {
				t.Fatal("managed normalization restored forbidden external XHTTP mux", out)
			}
			client, err := p.Outbound()
			if err != nil || client["streamSettings"].(map[string]any)["sockopt"].(map[string]any)["tcpMptcp"] != false {
				t.Fatal("managed normalization changed the client policy", client, err)
			}
			after, err := json.Marshal(map[string]any{"stream": p.Stream, "mux": p.Mux})
			if err != nil || string(before) != string(after) {
				t.Fatal("managed projection changed raw endpoint settings", err)
			}
		})
	}
}

func TestManagedRoutingStillRejectsRealSocketOverridesAndDialerChains(t *testing.T) {
	for key, value := range map[string]any{"tcpFastOpen": true, "mark": 7, "dialerProxy": "foreign-hop"} {
		t.Run(key, func(t *testing.T) {
			p := routingPolicyProxy()
			p.EndpointTuning = &subscription.EndpointOptions{}
			p.Stream["sockopt"] = map[string]any{key: value}
			out, err := guardedRoutingOutbound(p)
			if err == nil && routingPolicyNativeGuard(out) == nil {
				t.Fatal("neutral policy removal weakened the native socket guard", out)
			}
		})
	}
}

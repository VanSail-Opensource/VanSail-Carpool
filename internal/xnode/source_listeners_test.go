package xnode

import (
	"encoding/json"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Hysteria is the non-REALITY listener family whose clients are commonly
// direct Chinese broadband users. It must stay outside the country source
// filter so probes are handled by the node's probe protection instead.
func TestSourceGeoIPSkipsHysteriaListeners(t *testing.T) {
	native := json.RawMessage(`{
		"inbounds": [
			{"tag":"ss","listen":"0.0.0.0","port":1145,"protocol":"shadowsocks","settings":{"network":"tcp,udp"}},
			{"tag":"hy2","listen":"0.0.0.0","port":10000,"protocol":"hysteria","settings":{"version":2}},
			{"tag":"reality","listen":"0.0.0.0","port":443,"protocol":"vless","streamSettings":{"security":"reality"}}
		],
		"outbounds": []
	}`)
	listeners, err := sourceGeoIPListeners(native)
	if err != nil {
		t.Fatal(err)
	}
	want := []wire.SourceGeoIPListener{
		{ListenIP: "0.0.0.0", Port: 1145, Network: "tcp"},
		{ListenIP: "0.0.0.0", Port: 1145, Network: "udp"},
	}
	if len(listeners) != len(want) {
		t.Fatalf("listeners = %#v", listeners)
	}
	for index, item := range listeners {
		if item != want[index] {
			t.Fatalf("listener %d = %#v, want %#v", index, item, want[index])
		}
	}
}

func TestWithoutHysteriaInboundsPreservesOtherFields(t *testing.T) {
	native := json.RawMessage(`{"dns":{"servers":[]},"inbounds":[{"tag":"hy2","protocol":"hysteria","port":10000}],"routing":{}}`)
	filtered, err := withoutHysteriaInbounds(native)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err = json.Unmarshal(filtered, &root); err != nil {
		t.Fatal(err)
	}
	if root["dns"] == nil || root["routing"] == nil {
		t.Fatalf("unrelated sections were dropped: %s", filtered)
	}
	if inbounds, _ := root["inbounds"].([]any); len(inbounds) != 0 {
		t.Fatalf("hysteria inbound survived: %s", filtered)
	}
}

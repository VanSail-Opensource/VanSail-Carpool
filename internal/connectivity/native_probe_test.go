package connectivity

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestNativeOutboundProbeWithOpaquePublishedEndpoint(t *testing.T) {
	server, probe := trustedProbeTarget(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	address, authenticated := nativeVLESSFixture(t, probeUserUUID, server.URL)
	legacy := probeResolvedTarget(t, address, probeUserUUID)
	outbound, err := ClientOutbound(legacy)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(outbound)
	if err != nil {
		t.Fatal(err)
	}
	target := Target{HostUUID: "native-published-endpoint", Address: legacy.Address, Port: legacy.Port, NativeOutbound: raw}
	if err := validateSourceResult(42, User{ID: 42, Username: "monitor", Status: "ACTIVE"}, []Target{target}); err != nil {
		t.Fatal(err)
	}
	outcome := probe.Check(context.Background(), target, Config{ProbeURL: server.URL, TimeoutSeconds: 3})
	if outcome.Status != "connected" || authenticated.Load() != 1 {
		t.Fatal("native target did not traverse authenticated endpoint", outcome, authenticated.Load())
	}
}

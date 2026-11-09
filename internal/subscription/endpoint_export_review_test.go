package subscription

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"testing"
)

func endpointReviewProxy() Proxy {
	return Proxy{ID: "endpoint-a", NodeID: "node", InboundTag: "in", PrincipalID: 7, Name: "Review", Protocol: "vless", Address: "proxy.example", Port: 443, Account: map[string]any{"id": "c1d50c69-2f17-41ec-a23c-b30eb375639f", "encryption": "none"}, Stream: map[string]any{"network": "xhttp", "security": "tls", "tlsSettings": map[string]any{"serverName": "front.example", "fingerprint": "chrome"}, "xhttpSettings": map[string]any{"path": "/session", "mode": "packet-up"}}}
}

func TestEndpointAdminOptionsPrecedeMemberOverlayInNativeAndXray(t *testing.T) {
	p := endpointReviewProxy()
	admin, member := "firefox", "chrome"
	p.EndpointTuning = &EndpointOptions{Fingerprint: &admin}
	p.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{"xrayJson": []any{map[string]any{"op": "set", "to": "streamSettings.tlsSettings.fingerprint", "value": "edge"}}}}}
	for _, override := range []bool{false, true} {
		want := admin
		if override {
			want, p.Tuning = member, &ConnectionOverrides{Fingerprint: &member}
		}
		out, err := p.Outbound()
		if err != nil || text(object(object(out, "streamSettings"), "tlsSettings"), "fingerprint") != want {
			t.Fatal("Xray ownership order", out, err)
		}
		raw, err := p.NativeLink()
		if err != nil {
			t.Fatal(err)
		}
		link, err := url.Parse(raw)
		if err != nil || link.Query().Get("fp") != want {
			t.Fatal("native ownership order", raw, err)
		}
	}
}

func TestEndpointXHTTPExtraReplacesFlatHeadersWithoutMutatingSource(t *testing.T) {
	p := endpointReviewProxy()
	xhttp := object(p.Stream, "xhttpSettings")
	xhttp["headers"] = map[string]any{"Flat-Ignored": "wrong"}
	xhttp["extra"] = map[string]any{"headers": map[string]any{"Existing": "retained"}, "xmux": map[string]any{"maxConnections": 2}}
	headers := []TuningHeader{{Name: "Added", Value: "current"}}
	p.EndpointTuning = &EndpointOptions{XhttpHeaders: &headers}
	before, _ := json.Marshal(p.Stream)
	for _, format := range []string{"xray", "clash"} {
		out, err := p.endpointDocument(format)
		if err != nil {
			t.Fatal(format, err)
		}
		got := object(object(out, "xhttp-opts"), "headers")
		if format == "xray" {
			got = object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "headers")
		}
		if got["Flat-Ignored"] != nil || got["Existing"] != "retained" || got["Added"] != "current" {
			t.Fatal("whole-extra header precedence", format, got)
		}
	}
	after, _ := json.Marshal(p.Stream)
	if string(before) != string(after) {
		t.Fatal("export mutated administrator state")
	}
}

func TestEndpointSessionIDFieldsMapAndSplitEligibilityUsesActualNames(t *testing.T) {
	p := endpointReviewProxy()
	placement, key := "header", "X-Session"
	p.EndpointTuning = &EndpointOptions{XhttpSessionIDPlacement: &placement, XhttpSessionIDKey: &key}
	out, err := p.yamlProxy("clash")
	if err != nil || object(out, "xhttp-opts")["session-placement"] != placement || object(out, "xhttp-opts")["session-key"] != key {
		t.Fatal("sessionID mapping", out, err)
	}
	up, down := endpointReviewProxy(), endpointReviewProxy()
	up.ID, down.ID = "up", "down"
	object(up.Stream, "xhttpSettings")["extra"] = map[string]any{"sessionIDKey": "up-key"}
	object(down.Stream, "xhttpSettings")["extra"] = map[string]any{"sessionIDKey": "down-key"}
	if len(SplitModes(up, down)) != 0 {
		t.Fatal("incompatible native sessionID placement was paired")
	}
	object(down.Stream, "xhttpSettings")["extra"] = map[string]any{"sessionIDKey": "up-key"}
	if !slices.Contains(SplitModes(up, down), "packet-up") {
		t.Fatal("matching two-leg session settings rejected")
	}
}

func TestEndpointStaleDownloadAndProtocolOptionsRemainRenderable(t *testing.T) {
	p := endpointReviewProxy()
	enabled, oldPath := true, "/obsolete"
	p.EndpointTuning = &EndpointOptions{DownloadEnabled: &enabled, XhttpPath: &oldPath}
	p.Stream = map[string]any{"network": "ws", "security": "tls", "tlsSettings": map[string]any{"serverName": "front.example"}, "wsSettings": map[string]any{"path": "/current"}}
	out, err := p.Outbound()
	if err != nil || object(object(out, "streamSettings"), "xhttpSettings") != nil || text(object(object(out, "streamSettings"), "wsSettings"), "path") != "/current" {
		t.Fatal("stale XHTTP intent changed current protocol", out, err)
	}
	compat := endpointClientCompatibility(p, "xray")
	if !compat.Supported || !slices.Contains(compat.OmittedOptions, "downloadEnabled") || !slices.Contains(compat.OmittedOptions, "xhttpPath") {
		t.Fatal("stale options were not explained", compat)
	}
	raw, err := p.NativeLink()
	if err != nil {
		t.Fatal(err)
	}
	link, err := url.Parse(raw)
	if err != nil || link.Query().Get("path") != "/current" {
		t.Fatal("stale XHTTP path erased the current WebSocket path", raw, err)
	}
}

func TestEndpointDownloadDisableOverridesLegacyClientMapper(t *testing.T) {
	p := endpointReviewProxy()
	disabled := false
	p.EndpointTuning = &EndpointOptions{DownloadEnabled: &disabled}
	for _, format := range []string{"clash", "stash"} {
		mapper := "mihomo"
		if format == "stash" {
			mapper = "stash"
		}
		p.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{mapper: []any{map[string]any{"op": "set", "to": "xhttp-opts.download-settings.server", "value": "old.example"}}}}}
		out, err := p.yamlProxy(format)
		if err != nil || object(object(out, "xhttp-opts"), "download-settings") != nil {
			t.Fatal("legacy mapper restored explicitly disabled download leg", format, out, err)
		}
	}
}

func TestEndpointInheritedModeChangesOmitStaleDependentOptions(t *testing.T) {
	for _, scenario := range []string{"GET", "download"} {
		p := endpointReviewProxy()
		key := "xhttpUplinkHTTPMethod"
		if scenario == "GET" {
			method := "GET"
			p.EndpointTuning = &EndpointOptions{XhttpUplinkHTTPMethod: &method}
			object(p.Stream, "xhttpSettings")["mode"] = "stream-up"
		} else {
			enabled, address := true, "old-download.example"
			p.EndpointTuning = &EndpointOptions{DownloadEnabled: &enabled, DownloadAddress: &address}
			object(p.Stream, "xhttpSettings")["mode"] = "stream-one"
			key = "downloadEnabled"
		}
		out, err := p.Outbound()
		extra := effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings"))
		if err != nil || extra["uplinkHTTPMethod"] != nil || extra["downloadSettings"] != nil {
			t.Fatal("stale dependent option broke current mode", scenario, out, err)
		}
		compat := endpointClientCompatibility(p, "xray")
		if !compat.Supported || !slices.Contains(compat.OmittedOptions, key) {
			t.Fatal("stale dependency was not explained", scenario, compat)
		}
	}
}

func TestEndpointNewDownloadLegHasDestinationAndMatchingSessionDefaults(t *testing.T) {
	p := endpointReviewProxy()
	enabled := true
	p.EndpointTuning = &EndpointOptions{DownloadEnabled: &enabled}
	object(p.Stream, "xhttpSettings")["extra"] = map[string]any{"sessionIDPlacement": "header", "sessionIDKey": "X-SID"}
	out, err := p.Outbound()
	down := object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "downloadSettings")
	if err != nil || text(down, "address") != p.Address || fmt.Sprint(down["port"]) != fmt.Sprint(p.Port) {
		t.Fatal("new download leg lacks a dial destination", down, err)
	}
	settings := object(down, "xhttpSettings")
	if text(settings, "path") != "/session" || text(effectiveXHTTP(settings), "sessionIDKey") != "X-SID" || object(effectiveXHTTP(settings), "downloadSettings") != nil {
		t.Fatal("new download leg lost listener/session identity or recursed", down)
	}
}

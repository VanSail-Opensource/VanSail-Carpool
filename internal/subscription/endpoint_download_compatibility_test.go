package subscription

import (
	"encoding/json"
	"net/url"
	"slices"
	"testing"
)

func TestEndpointInactiveInheritedDownloadCannotExcludeCurrentConnection(t *testing.T) {
	for _, scenario := range []string{"disabled", "other-transport", "stream-one"} {
		t.Run(scenario, func(t *testing.T) {
			p := endpointReviewProxy()
			p.EndpointTuning = &EndpointOptions{}
			object(p.Stream, "xhttpSettings")["extra"] = map[string]any{"downloadSettings": map[string]any{"network": "grpc", "address": "old-download.example", "port": 443}}
			if scenario == "disabled" {
				disabled := false
				p.EndpointTuning.DownloadEnabled = &disabled
			} else if scenario == "other-transport" {
				p.Stream["network"] = "ws"
				p.Stream["wsSettings"] = map[string]any{"path": "/current"}
			} else {
				object(p.Stream, "xhttpSettings")["mode"] = "stream-one"
			}
			before, _ := json.Marshal(p.Stream)
			for _, format := range []string{"xray", "native"} {
				compat := endpointClientCompatibility(p, format)
				if !compat.Supported || slices.Contains(compat.Reasons, "downloadTransport") {
					t.Fatal("inactive download became an essential exclusion", format, compat)
				}
				if scenario == "other-transport" && !slices.Contains(compat.Reasons, "context") || scenario == "stream-one" && !slices.Contains(compat.Reasons, "dependency") {
					t.Fatal("inactive legacy download was not explained", compat)
				}
			}
			out, err := p.Outbound()
			if err != nil {
				t.Fatal("repair or current endpoint remained unrenderable", out, err)
			}
			if scenario != "other-transport" && object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "downloadSettings") != nil {
				t.Fatal("inactive download remained in active XHTTP stream", out)
			}
			raw, err := p.NativeLink()
			if err != nil {
				t.Fatal(raw, err)
			}
			if scenario == "other-transport" {
				link, parseErr := url.Parse(raw)
				if parseErr != nil || link.Query().Get("path") != "/current" {
					t.Fatal("stale download altered active WebSocket path", raw, parseErr)
				}
			}
			after, _ := json.Marshal(p.Stream)
			if string(before) != string(after) {
				t.Fatal("compatibility or repair rewrote raw inherited settings")
			}
		})
	}
}

func TestEndpointActiveUnsupportedDownloadStillExcludesOnlyThatPeer(t *testing.T) {
	p, peer := endpointReviewProxy(), endpointReviewProxy()
	p.EndpointTuning, peer.EndpointTuning = &EndpointOptions{}, &EndpointOptions{}
	peer.ID = "compatible-peer"
	object(p.Stream, "xhttpSettings")["extra"] = map[string]any{"downloadSettings": map[string]any{"network": "grpc", "address": "download.example", "port": 443}}
	compat := endpointClientCompatibility(p, "xray")
	if compat.Supported || !slices.Contains(compat.Reasons, "downloadTransport") {
		t.Fatal("active essential incompatibility was lost", compat)
	}
	selected, excluded, err := CompatibleProxies("xray", []Proxy{p, peer})
	if err != nil || len(selected) != 1 || selected[0].ID != peer.ID || excluded != 1 {
		t.Fatal("essential incompatibility affected the safe peer", selected, excluded, err)
	}
}

func TestEndpointClientMetadataAgreesWithRealAdapterCapabilities(t *testing.T) {
	for _, scenario := range []string{"vmess-xhttp", "vless-kcp", "invalid-mapper"} {
		for _, format := range []string{"clash", "stash"} {
			t.Run(scenario+"/"+format, func(t *testing.T) {
				p, peer := endpointReviewProxy(), endpointReviewProxy()
				p.EndpointTuning, peer.EndpointTuning = &EndpointOptions{}, &EndpointOptions{}
				peer.ID = "valid-peer"
				if scenario == "vmess-xhttp" {
					p.Protocol = "vmess"
				} else if scenario == "vless-kcp" {
					p.Stream["network"] = "kcp"
				} else {
					mapper := "mihomo"
					if format == "stash" {
						mapper = "stash"
					}
					p.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{mapper: []any{map[string]any{"op": "unknown", "to": "name"}}}}}
				}
				compat := endpointClientCompatibility(p, format)
				if compat.Supported || !slices.Contains(compat.Reasons, "format") {
					t.Fatal("metadata advertised unsupported base adapter", compat)
				}
				selected, excluded, err := CompatibleProxies(format, []Proxy{p, peer})
				if err != nil || len(selected) != 1 || selected[0].ID != peer.ID || excluded != 1 {
					t.Fatal("base adapter failure affected compatible peers", selected, excluded, err)
				}
			})
		}
	}
}

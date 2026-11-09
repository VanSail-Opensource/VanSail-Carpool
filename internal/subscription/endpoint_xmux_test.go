package subscription

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	xrayconf "github.com/xtls/xray-core/infra/conf"
)

func xmuxReviewProxy(parentKey string, parent int, child map[string]any, options EndpointOptions) Proxy {
	p := endpointReviewProxy()
	p.EndpointTuning = &options
	extra := map[string]any{"xmux": map[string]any{parentKey: parent, "cMaxReuseTimes": 21, "hMaxRequestTimes": 61, "hKeepAlivePeriod": 37}}
	if child != nil {
		extra["downloadSettings"] = map[string]any{"address": "down.example", "port": 443, "network": "xhttp", "security": "tls", "tlsSettings": map[string]any{"serverName": "down.example"}, "xhttpSettings": map[string]any{"extra": map[string]any{"xmux": child}}}
	}
	object(p.Stream, "xhttpSettings")["extra"] = extra
	return p
}

func buildXMUXReview(t *testing.T, p Proxy) map[string]any {
	t.Helper()
	out, err := p.Outbound()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(object(out, "streamSettings"))
	if err != nil {
		t.Fatal(err)
	}
	var stream xrayconf.StreamConfig
	if err = json.Unmarshal(raw, &stream); err != nil {
		t.Fatal(err)
	}
	if _, err = stream.Build(); err != nil {
		t.Fatal("materialized XMUX failed pinned native build", err)
	}
	return out
}

func TestEndpointSingleTypedXMUXChoiceReplacesOpposingInheritedMode(t *testing.T) {
	for _, download := range []bool{false, true} {
		for _, connections := range []bool{false, true} {
			options := EndpointOptions{}
			enabled := true
			rangeValue := &TuningRange{Min: 8, Max: 8}
			parent, own, other := "maxConnections", "maxConcurrency", "maxConnections"
			if connections {
				parent, own, other = "maxConcurrency", "maxConnections", "maxConcurrency"
			}
			if download {
				options.DownloadEnabled = &enabled
				if connections {
					options.DownloadXmuxMaxConnections = rangeValue
				} else {
					options.DownloadXmuxMaxConcurrency = rangeValue
				}
			} else if connections {
				options.XmuxMaxConnections = rangeValue
			} else {
				options.XmuxMaxConcurrency = rangeValue
			}
			p := xmuxReviewProxy(parent, 16, nil, options)
			before, _ := json.Marshal(p.Stream)
			if errors := ValidateEndpointOptions(options, p); len(errors) != 0 {
				t.Fatal("single explicit mode should be usable", errors)
			}
			out := buildXMUXReview(t, p)
			settings := effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings"))
			if download {
				settings = effectiveXHTTP(object(object(settings, "downloadSettings"), "xhttpSettings"))
			}
			mux := object(settings, "xmux")
			if fmt.Sprint(mux[own]) != "8" || xmuxMaximum(mux[other]) != 0 || fmt.Sprint(mux["cMaxReuseTimes"]) != "21" {
				t.Fatal("typed mode lost precedence or other XMUX controls", download, mux)
			}
			ownYAML, otherYAML := "max-concurrency", "max-connections"
			if connections {
				ownYAML, otherYAML = otherYAML, ownYAML
			}
			prefix := "xhttp-opts."
			if download {
				prefix += "download-settings."
			}
			p.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{"mihomo": []any{map[string]any{"op": "set", "to": prefix + "reuse-settings." + otherYAML, "value": 99}}}}}
			yaml, err := p.yamlProxy("clash")
			clientOptions := object(yaml, "xhttp-opts")
			if download {
				clientOptions = object(clientOptions, "download-settings")
			}
			reuse := object(clientOptions, "reuse-settings")
			if err != nil || fmt.Sprint(reuse[ownYAML]) != "8" || xmuxMaximum(reuse[otherYAML]) != 0 || fmt.Sprint(reuse["c-max-reuse-times"]) != "21" || fmt.Sprint(reuse["h-keep-alive-period"]) != "37" {
				t.Fatal("Mihomo typed choice or unrelated fields lost after mapper", yaml, err)
			}
			after, _ := json.Marshal(p.Stream)
			if string(before) != string(after) {
				t.Fatal("typed XMUX merge changed raw state")
			}
		}
	}
}

func TestEndpointTwoExplicitXMUXModesConflictAtSaveButRemainSafeAtRuntime(t *testing.T) {
	for _, download := range []bool{false, true} {
		positive := &TuningRange{Min: 8, Max: 8}
		options := EndpointOptions{XmuxMaxConcurrency: positive, XmuxMaxConnections: positive}
		key := "xmuxMaxConnections"
		if download {
			enabled := true
			options = EndpointOptions{DownloadEnabled: &enabled, DownloadXmuxMaxConcurrency: positive, DownloadXmuxMaxConnections: positive}
			key = "downloadXmuxMaxConnections"
		}
		p := xmuxReviewProxy("maxConcurrency", 16, nil, options)
		if errors := ValidateEndpointOptions(options, p); errors[key] != "conflict" {
			t.Fatal("explicit conflicting pair was accepted", errors)
		}
		for _, format := range []string{"xray", "clash"} {
			compat := endpointClientCompatibility(p, format)
			if !compat.Supported || !slices.Contains(compat.Reasons, "conflict") || !slices.Contains(compat.OmittedOptions, key) {
				t.Fatal("cached conflicting optional choices excluded endpoint or hid reason", compat)
			}
			peer := endpointReviewProxy()
			peer.ID, peer.EndpointTuning = "safe-peer", &EndpointOptions{}
			selected, excluded, err := CompatibleProxies(format, []Proxy{p, peer})
			if err != nil || len(selected) != 2 || excluded != 0 {
				t.Fatal("optional choice conflict affected usable peers", selected, excluded, err)
			}
		}
		buildXMUXReview(t, p)
	}
}

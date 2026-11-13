package subscription

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestEndpointLegacyInsecureTLSProjectsSecureWithoutRewritingSource(t *testing.T) {
	p := endpointReviewProxy()
	p.EndpointTuning = &EndpointOptions{}
	p.Stream["network"] = "ws"
	p.Stream["wsSettings"] = map[string]any{"path": "/current"}
	delete(p.Stream, "xhttpSettings")
	object(p.Stream, "tlsSettings")["allowInsecure"] = true
	before, _ := json.Marshal(p.Stream)
	if failures := ValidateEndpointOptions(EndpointOptions{}, p); len(failures) != 0 {
		t.Fatal("legacy endpoint cannot be repaired", failures)
	}
	for _, format := range []string{"native", "base64", "xray", "sing-box", "clash", "stash"} {
		compat := endpointClientCompatibility(p, format)
		if !compat.Supported || !slices.Contains(compat.Reasons, "fixedOff") || !slices.Contains(compat.OmittedOptions, "tlsAllowInsecure") {
			t.Fatal(format, compat)
		}
		selected, excluded, err := CompatibleProxies(format, []Proxy{p})
		if err != nil || len(selected) != 1 || excluded != 0 {
			t.Fatal("fixed-off compatibility excluded endpoint", format, selected, excluded, err)
		}
	}
	out, err := p.Outbound()
	if err != nil || object(object(out, "streamSettings"), "tlsSettings")["allowInsecure"] != false {
		t.Fatal("native policy did not force secure false", out, err)
	}
	after, _ := json.Marshal(p.Stream)
	if string(before) != string(after) {
		t.Fatal("legacy encrypted-source projection was mutated")
	}
}

func TestEndpointLegacyInsecureDownloadTLSIsAlsoFixedOff(t *testing.T) {
	p := endpointReviewProxy()
	p.EndpointTuning = &EndpointOptions{}
	object(p.Stream, "xhttpSettings")["extra"] = map[string]any{"downloadSettings": map[string]any{
		"address": "down.example", "port": 443, "network": "xhttp", "security": "tls", "tlsSettings": map[string]any{"allowInsecure": true},
	}}
	if failures := ValidateEndpointOptions(EndpointOptions{}, p); len(failures) != 0 {
		t.Fatal("legacy download endpoint cannot be repaired", failures)
	}
	for _, format := range []string{"xray", "clash", "stash"} {
		out, err := p.endpointDocument(format)
		if err != nil {
			t.Fatal(format, err)
		}
		var insecure any
		if format == "xray" {
			down := object(effectiveXHTTP(object(object(out, "streamSettings"), "xhttpSettings")), "downloadSettings")
			insecure = object(down, "tlsSettings")["allowInsecure"]
		} else {
			insecure = object(object(out, "xhttp-opts"), "download-settings")["skip-cert-verify"]
		}
		if insecure != false {
			t.Fatal("download policy did not force secure false", format, out)
		}
	}
}

func TestEndpointLegacyMapperCannotRestoreInsecureClientTLS(t *testing.T) {
	for _, format := range []string{"xray", "sing-box", "clash", "stash"} {
		t.Run(format, func(t *testing.T) {
			p := endpointReviewProxy()
			p.EndpointTuning = &EndpointOptions{}
			mapper, path := "xrayJson", "streamSettings.tlsSettings.allowInsecure"
			if format == "sing-box" {
				mapper, path = "singbox", "tls.insecure"
				p.Stream["network"] = "ws"
				p.Stream["wsSettings"] = map[string]any{"path": "/current"}
			} else if yamlFormat(format) {
				mapper, path = "mihomo", "skip-cert-verify"
				if format == "stash" {
					mapper = "stash"
				}
				object(p.Stream, "xhttpSettings")["extra"] = map[string]any{"downloadSettings": map[string]any{"address": "down.example", "network": "xhttp", "security": "tls"}}
			}
			operations := []any{map[string]any{"op": "set", "to": path, "value": true}}
			if yamlFormat(format) {
				operations = append(operations, map[string]any{"op": "set", "to": "xhttp-opts.download-settings.skip-cert-verify", "value": true})
			}
			p.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{mapper: operations}}}
			before, err := json.Marshal(p.MapperHost)
			if err != nil {
				t.Fatal(err)
			}
			out, err := p.endpointDocument(format)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := tuningPathValue(out, path); value != false {
				t.Fatal("mapper bypassed fixed-off certificate policy", out)
			}
			if yamlFormat(format) && object(object(out, "xhttp-opts"), "download-settings")["skip-cert-verify"] != false {
				t.Fatal("download mapper bypassed fixed-off certificate policy", out)
			}
			after, err := json.Marshal(p.MapperHost)
			if err != nil || string(before) != string(after) {
				t.Fatal("client policy rewrote saved mapper", err)
			}
		})
	}
}

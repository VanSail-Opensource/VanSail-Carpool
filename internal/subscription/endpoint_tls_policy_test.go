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

package subscription

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestEndpointOptionalOmissionAndEssentialExclusionKeepCompatiblePeers(t *testing.T) {
	p, peer := endpointReviewProxy(), endpointReviewProxy()
	p.Stream["network"], peer.Stream["network"] = "tcp", "tcp"
	peer.ID = "endpoint-peer"
	peer.EndpointTuning = &EndpointOptions{}
	cipher := "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	p.EndpointTuning = &EndpointOptions{TlsCipherSuites: &cipher}
	selected, excluded, err := CompatibleProxies("clash", []Proxy{p, peer})
	if err != nil || len(selected) != 2 || excluded != 0 {
		t.Fatal("optional tuning excluded a usable endpoint", len(selected), excluded, err)
	}
	compat := endpointClientCompatibility(p, "clash")
	if !compat.Supported || !slices.Contains(compat.OmittedOptions, "tlsCipherSuites") {
		t.Fatal("optional omission was not reported", compat)
	}
	disableRoots := true
	p.EndpointTuning = &EndpointOptions{TlsDisableSystemRoot: &disableRoots}
	selected, excluded, err = CompatibleProxies("clash", []Proxy{p, peer})
	if err != nil || len(selected) != 1 || selected[0].ID != peer.ID || excluded != 1 {
		t.Fatal("essential exclusion affected compatible peer", selected, excluded, err)
	}
	selected, excluded, err = CompatibleProxies("xray", []Proxy{p, peer})
	if err != nil || len(selected) != 2 || excluded != 0 {
		t.Fatal("client supporting essential option lost endpoint", len(selected), excluded, err)
	}
}

func TestEndpointMemberDecoderCannotAcceptAdministratorOrRetiredFields(t *testing.T) {
	for _, raw := range []string{`{"hysteriaPorts":[{"min":443,"max":444}]}`, `{"xhttpSessionIDKey":"private"}`, `{"downloadEnabled":true}`, `{"xhttpNoGRPCHeader":true}`, `{"xmuxHKeepAlivePeriod":30}`} {
		var member ConnectionOverrides
		if err := json.Unmarshal([]byte(raw), &member); err == nil {
			t.Fatal("administrator option entered member allowlist", raw)
		}
	}
}

func TestEndpointDecoderRejectsUnknownNullAndInvalidBoundaries(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"unknown":true}`, `{"tlsServerName":null}`, `{"hysteriaPorts":null}`, `{"hysteriaHopInterval":{"min":null,"max":30}}`, `{"hysteriaHopInterval":{"min":5}}`, `{"hysteriaPorts":[{"min":1,"max":2,"unknown":3}]}`} {
		var endpoint EndpointOptions
		if err := json.Unmarshal([]byte(raw), &endpoint); err == nil {
			t.Fatal("untyped endpoint option accepted", raw)
		}
	}
	p := hoppingProxy()
	p.Stream = map[string]any{"network": "hysteria", "security": "tls", "tlsSettings": map[string]any{"serverName": "front.example"}, "hysteriaSettings": map[string]any{"version": 2, "auth": "synthetic"}}
	for _, raw := range []string{`{"hysteriaPorts":[{"min":0,"max":443}]}`, `{"hysteriaPorts":[{"min":443,"max":65536}]}`, `{"hysteriaPorts":[{"min":443,"max":442}]}`, `{"hysteriaPorts":[{"min":443,"max":444}],"hysteriaHopInterval":{"min":4,"max":30}}`} {
		var endpoint EndpointOptions
		if err := json.Unmarshal([]byte(raw), &endpoint); err != nil {
			t.Fatal("fixture shape", err)
		}
		field := "hysteriaPorts"
		if strings.Contains(raw, "hysteriaHopInterval") {
			field = "hysteriaHopInterval"
		}
		if failures := ValidateEndpointOptions(endpoint, p); failures[field] == "" {
			t.Fatal("invalid endpoint bounds did not identify their field", raw, failures)
		}
	}
}

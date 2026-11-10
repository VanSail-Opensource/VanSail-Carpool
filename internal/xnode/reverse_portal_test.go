package xnode

import (
	"encoding/json"
	"testing"
)

const reverseTestUUID = "11111111-1111-4111-8111-111111111111"

func reverseInbound(tag, reverseTag string) map[string]any {
	inbound := map[string]any{
		"tag": tag, "listen": "127.0.0.1", "port": 18443, "protocol": "vless",
		"settings":       map[string]any{"decryption": "none"},
		"streamSettings": map[string]any{"network": "tcp", "security": "none"},
	}
	if reverseTag != "" {
		inbound["reverseTag"] = reverseTag
	}
	return inbound
}

func reverseDocument(inbounds, outbounds []any) json.RawMessage {
	raw, err := json.Marshal(map[string]any{"inbounds": inbounds, "outbounds": outbounds})
	if err != nil {
		panic(err)
	}
	return raw
}

func TestVLESSReversePortalCredentialCarriesTag(t *testing.T) {
	credential, err := inboundCredential(reverseInbound("AH-TUNNEL", "reverse-hntw"), Credentials{UUID: reverseTestUUID})
	if err != nil {
		t.Fatal(err)
	}
	var account map[string]any
	if json.Unmarshal(credential, &account) != nil {
		t.Fatal("credential is not an object")
	}
	reverse, _ := account["reverse"].(map[string]any)
	if account["id"] != reverseTestUUID || reverse["tag"] != "reverse-hntw" {
		t.Fatalf("reverse portal account = %v", account)
	}
}

func TestVLESSReversePortalKeepsStoredCredentialFields(t *testing.T) {
	credentials := Credentials{Inbound: map[string]json.RawMessage{
		"AH-TUNNEL": json.RawMessage(`{"id":"` + reverseTestUUID + `","custom":"kept"}`),
	}}
	credential, err := inboundCredential(reverseInbound("AH-TUNNEL", "reverse-hntw"), credentials)
	if err != nil {
		t.Fatal(err)
	}
	var account map[string]any
	if json.Unmarshal(credential, &account) != nil {
		t.Fatal("credential is not an object")
	}
	reverse, _ := account["reverse"].(map[string]any)
	if account["custom"] != "kept" || reverse["tag"] != "reverse-hntw" {
		t.Fatalf("stored account was not preserved: %v", account)
	}
}

func TestVLESSReversePortalRejectsInvalidTag(t *testing.T) {
	for _, inbound := range []map[string]any{
		reverseInbound("AH-TUNNEL", "bad tag"),
		{"tag": "AH-SS", "protocol": "shadowsocks", "settings": map[string]any{"method": "chacha20-ietf-poly1305"}, "reverseTag": "reverse-hntw"},
	} {
		if _, err := inboundCredential(inbound, Credentials{UUID: reverseTestUUID}); err == nil {
			t.Fatalf("invalid reverse portal accepted: %v", inbound)
		}
	}
}

func TestReversePortalDocumentValidation(t *testing.T) {
	freedom := map[string]any{"tag": "direct", "protocol": "freedom"}
	valid := reverseDocument([]any{reverseInbound("AH-TUNNEL", "reverse-hntw")}, []any{freedom})
	if result := validateConfigurationDocument(valid, nil); !result.Valid {
		t.Fatalf("valid reverse portal rejected: %s", result.Detail)
	}
	for name, document := range map[string]json.RawMessage{
		"duplicate tag": reverseDocument([]any{
			reverseInbound("AH-TUNNEL", "reverse-hntw"),
			reverseInbound("AH-TUNNEL-2", "reverse-hntw"),
		}, []any{freedom}),
		"outbound collision": reverseDocument(
			[]any{reverseInbound("AH-TUNNEL", "direct")}, []any{freedom}),
		"non vless": reverseDocument([]any{map[string]any{
			"tag": "AH-SS", "protocol": "shadowsocks", "port": 1145,
			"settings": map[string]any{"method": "chacha20-ietf-poly1305"}, "reverseTag": "reverse-hntw",
		}}, []any{freedom}),
		"invalid name": reverseDocument([]any{reverseInbound("AH-TUNNEL", "bad tag")}, []any{freedom}),
	} {
		if result := validateConfigurationDocument(document, nil); result.Valid {
			t.Fatalf("%s accepted", name)
		}
	}
}

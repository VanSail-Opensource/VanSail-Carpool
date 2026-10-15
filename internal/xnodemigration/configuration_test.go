package xnodemigration

import (
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestMigrationPreservesNativeUtilitiesAndInactiveManagedSelection(t *testing.T) {
	document := json.RawMessage(`{"inbounds":[{"tag":"paid inbound","protocol":"vless","settings":{"decryption":"none","clients":[{"id":"old-managed"}]}},{"tag":"inactive","protocol":"vless","settings":{"decryption":"none"}},{"tag":"utility forward","protocol":"dokodemo-door","listen":"127.0.0.1","port":10001,"settings":{"address":"127.0.0.1","port":8080,"network":"tcp"}}],"outbounds":[{"tag":"direct","protocol":"freedom"}],"routing":{"rules":[{"type":"field","inboundTag":["utility forward","paid inbound"],"outboundTag":"direct"}]}}`)
	value, tags, err := NativeDocument("profile", document, Node{Inbounds: []remnawave.NodeInbound{{UUID: "paid-id", Tag: "paid inbound"}}})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err = json.Unmarshal(value, &root); err != nil {
		t.Fatal(err)
	}
	ins := root["inbounds"].([]any)
	if len(ins) != 2 {
		t.Fatalf("utility lost or inactive proxy enabled: %s", value)
	}
	paid, utility := ins[0].(map[string]any), ins[1].(map[string]any)
	if _, retained := paid["settings"].(map[string]any)["clients"]; retained || tags["paid-id"] != paid["tag"] {
		t.Fatalf("managed account/tag conversion: %s", value)
	}
	if utility["settings"].(map[string]any)["address"] != "127.0.0.1" {
		t.Fatal("utility destination changed")
	}
	rules := root["routing"].(map[string]any)["rules"].([]any)
	refs := rules[1].(map[string]any)["inboundTag"].([]any)
	if refs[0] != utility["tag"] || refs[1] != paid["tag"] {
		t.Fatalf("routing references lost: %s", value)
	}
	if _, review := root["MigrationReview"]; review {
		t.Fatal("supported utility marked as unsupported")
	}
}

func TestUnsupportedSourceUtilityIsRetainedAndBlocksActivationForReview(t *testing.T) {
	document := json.RawMessage(`{"inbounds":[{"tag":"service-http","protocol":"http","listen":"127.0.0.1","port":10080,"settings":{"accounts":[{"user":"service","pass":"keep-secret"}]}}],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`)
	value, _, err := NativeDocument("profile", document, Node{})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	_ = json.Unmarshal(value, &root)
	if root["MigrationReview"] == nil {
		t.Fatal("unsupported utility could activate silently")
	}
	if xnode.ValidateConfigurationDocument(value).Valid {
		t.Fatal("review marker did not prevent native activation")
	}
	inbound := root["inbounds"].([]any)[0].(map[string]any)
	account := inbound["settings"].(map[string]any)["accounts"].([]any)[0].(map[string]any)
	if account["pass"] != "keep-secret" {
		t.Fatal("source service credential was discarded")
	}
	preview := inspectSnapshot(Snapshot{Profiles: []remnawave.MigrationProfile{{UUID: "profile", Config: document}}})
	if len(preview.Issues) != 1 || !preview.Issues[0].Blocking || preview.Issues[0].Code != "UNSUPPORTED_UTILITY_INBOUND" {
		t.Fatalf("missing conversion warning: %+v", preview)
	}
}

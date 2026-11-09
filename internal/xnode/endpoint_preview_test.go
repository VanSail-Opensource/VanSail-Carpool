package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestEndpointPreviewIsReadOnlyAndOptionsRemainEncrypted(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	draft := xnode.Endpoint{Name: "Published", NodeID: f.node.ID, InboundTag: "paid", Address: "host.example", Port: 443, Enabled: true,
		Overrides: json.RawMessage(`{"clientOptions":{"tcpFastOpen":true}}`)}
	before, _, err := f.service.Node(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := f.service.PreviewEndpoint(ctx, draft)
	if err != nil || !preview.Valid || preview.Metadata.Protocol != "vless" || preview.Metadata.Transport != "tcp" || len(preview.Clients) != 6 {
		t.Fatal(preview, err)
	}
	encoded, _ := json.Marshal(preview)
	var responseFields map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &responseFields); err != nil || len(responseFields) != 6 {
		t.Fatal("preview response changed its safe metadata boundary", err)
	}
	for _, field := range []string{"valid", "metadata", "inbounds", "fields", "fieldErrors", "clients"} {
		if _, ok := responseFields[field]; !ok {
			t.Fatal("preview response returned an unexpected field instead of", field)
		}
	}
	if strings.Contains(string(encoded), "00000000-0000-4000-8000-000000000001") || strings.Contains(string(encoded), "synthetic-preview") {
		t.Fatal("preview exposed credentials", string(encoded))
	}
	rows, err := f.store.XNodeEndpoints(ctx)
	after, _, nodeErr := f.service.Node(ctx, f.node.ID)
	if err != nil || nodeErr != nil || len(rows) != 0 || before.DesiredRevision != after.DesiredRevision {
		t.Fatal("preview changed desired state", rows, err, nodeErr)
	}
	saved, err := f.service.SaveEndpoint(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := f.store.XNodeEndpoints(ctx)
	if err != nil || len(stored) != 1 || strings.Contains(stored[0].OverridesCipher, "tcpFastOpen") || stored[0].OverridesCipher == string(draft.Overrides) {
		t.Fatal("plaintext endpoint options persisted", stored, err)
	}
	loaded, err := f.service.AdminEndpoints(ctx)
	if err != nil || len(loaded) != 1 || string(loaded[0].Overrides) != string(draft.Overrides) || loaded[0].ID != saved.ID || loaded[0].Metadata.Protocol != "vless" {
		t.Fatal("encrypted endpoint round trip failed", loaded, err)
	}
}

func TestEndpointSaveRevalidatesProtocolChangesAndIncompleteDrafts(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	profile, err := f.service.ResolveConfiguration(ctx, f.node.ConfigID)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if json.Unmarshal(profile.Document, &native) != nil {
		t.Fatal("fixture document")
	}
	inbound := native["inbounds"].([]any)[0].(map[string]any)
	inbound["streamSettings"] = map[string]any{"network": "xhttp", "xhttpSettings": map[string]any{"path": "/client"}}
	profile.Document, _ = json.Marshal(native)
	if profile, err = f.service.SaveConfiguration(ctx, profile); err != nil {
		t.Fatal(err)
	}
	draft := xnode.Endpoint{Name: "XHTTP", NodeID: f.node.ID, InboundTag: "paid", Address: "host.example", Port: 443, Overrides: json.RawMessage(`{"clientOptions":{"xhttpMode":"auto"}}`)}
	preview, err := f.service.PreviewEndpoint(ctx, draft)
	if err != nil || !preview.Valid || preview.Metadata.Transport != "xhttp" {
		t.Fatal(preview, err)
	}
	// A configuration edit between preview and save cannot persist stale options.
	inbound["streamSettings"] = map[string]any{"network": "ws", "wsSettings": map[string]any{"path": "/ws"}}
	profile.Document, _ = json.Marshal(native)
	if profile, err = f.service.SaveConfiguration(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.SaveEndpoint(ctx, draft); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatal("stale XHTTP options accepted for WebSocket", err)
	}
	draft.Overrides = json.RawMessage(`{}`)
	draft.Name, draft.Address, draft.Port = "", "", 0
	preview, err = f.service.PreviewEndpoint(ctx, draft)
	if err != nil || preview.Valid || preview.Metadata.Transport != "ws" || len(preview.Fields) == 0 || len(preview.FieldErrors) != 3 {
		t.Fatal("incomplete draft lost its derived controls", preview, err)
	}
}

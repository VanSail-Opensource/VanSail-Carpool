package xnode_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestDIYSettingsRoundTripConcurrentRetriesAndExportIsolation(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	c := iplookup.DefaultConfig()
	c.LookupFeeTXB = "0"
	if err := f.store.SaveIPLookupSettings(ctx, f.user.ID, c, nil, nil); err != nil {
		t.Fatal(err)
	}
	view, err := f.service.OwnedDIY(ctx, f.user.ID)
	if err != nil || view.SettingsRevision != 0 || len(view.Fields) != len(subscription.TuningFields()) {
		t.Fatal("initial inheritance", view, err)
	}
	var overrides subscription.ConnectionOverrides
	if err = json.Unmarshal([]byte(`{"xhttpXPaddingBytes":{"min":0,"max":0},"tcpFastOpen":false,"muxXudpConcurrency":-1}`), &overrides); err != nil {
		t.Fatal(err)
	}
	input := xnode.DIYWrite{Quote: view.Billing.Token, SettingsRevision: 0, RoutingRevision: view.Routing.Plan.Revision, Overrides: overrides, Choices: []xnode.RoutingChoice{}}
	preview, err := f.service.PreviewOwnedDIY(ctx, f.user.ID, input)
	if err != nil || !preview.Valid {
		t.Fatal("draft preview", preview, err)
	}
	results := make([]xnode.DIYSaveResult, 2)
	errors := make([]error, 2)
	var group sync.WaitGroup
	for i := range results {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			results[i], errors[i] = f.service.SaveOwnedDIY(ctx, f.user.ID, "same-request", input)
		}(i)
	}
	group.Wait()
	if errors[0] != nil || errors[1] != nil || results[0].Receipt.ID != results[1].Receipt.ID || results[0].RoutingChanged {
		t.Fatal("concurrent replay", results, errors)
	}
	view, err = f.service.OwnedDIY(ctx, f.user.ID)
	raw, _ := json.Marshal(view.Overrides)
	if err != nil || view.SettingsRevision != 1 || string(raw) != `{"xhttpXPaddingBytes":{"min":0,"max":0},"tcpFastOpen":false,"muxXudpConcurrency":-1}` {
		t.Fatal("explicit values", string(raw), err)
	}
	doc, err := f.store.XNodeDIY(ctx, f.user.ID)
	if err != nil || strings.Contains(doc.Cipher, "tcpFastOpen") {
		t.Fatal("plaintext persistence", err)
	}
	base := []subscription.Proxy{{ID: "synthetic", Protocol: "vless", Account: map[string]any{"id": "private"}}}
	exported, err := f.service.ExportProxiesForUser(ctx, f.user.ID, base)
	if err != nil || exported[0].Tuning == nil || base[0].Tuning != nil {
		t.Fatal("tuning escaped export boundary", err)
	}
	other, err := f.store.XNodeDIY(ctx, "another-owner")
	if err != nil || other.Revision != 0 || other.Cipher != "" {
		t.Fatal("owner document leaked", other, err)
	}
	input.SettingsRevision = 1
	input.Quote = view.Billing.Token
	input.Overrides = subscription.ConnectionOverrides{}
	if _, err = f.service.SaveOwnedDIY(ctx, f.user.ID, "inherit", input); err != nil {
		t.Fatal(err)
	}
	exported, err = f.service.ExportProxiesForUser(ctx, f.user.ID, base)
	if err != nil || exported[0].Tuning == nil || base[0].Tuning != nil {
		t.Fatal("inheritance reset", err)
	}
	// Empty overrides still opt member exports into the fixed client policy.
	raw, err = json.Marshal(exported[0].Tuning)
	if err != nil || string(raw) != `{}` {
		t.Fatal("inheritance retained overrides", string(raw), err)
	}
}

func TestDIYGeneratedXrayUsesPinnedParserAndAdministratorIdentity(t *testing.T) {
	var options subscription.ConnectionOverrides
	if err := json.Unmarshal([]byte(`{"fingerprint":"firefox","tcpFastOpen":false,"muxEnabled":false,"xmuxMaxConcurrency":{"min":1,"max":2}}`), &options); err != nil {
		t.Fatal(err)
	}
	p := subscription.Proxy{ID: "host", Name: "Fixture", Protocol: "vless", Address: "server.example", Port: 443, Account: map[string]any{"id": "c1d50c69-2f17-41ec-a23c-b30eb375639f", "encryption": "none"}, Stream: map[string]any{"network": "xhttp", "security": "tls", "tlsSettings": map[string]any{"serverName": "admin.example", "fingerprint": "chrome"}, "xhttpSettings": map[string]any{"path": "/admin", "mode": "auto"}}, Tuning: &options}
	raw, err := subscription.RenderXray([]subscription.Proxy{p})
	if err != nil {
		t.Fatal(err)
	}
	var documents []json.RawMessage
	if err = json.Unmarshal(raw, &documents); err != nil || len(documents) != 1 {
		t.Fatal("Xray document array", err)
	}
	for _, document := range documents {
		if validation := xnode.ValidateClientConfigurationDocument(document); !validation.Valid {
			t.Fatal("pinned parser", validation)
		}
	}
	if !strings.Contains(string(raw), "admin.example") || !strings.Contains(string(raw), "/admin") || strings.Contains(string(raw), "privateKey") {
		t.Fatal("protected transport identity", string(raw))
	}
}

package xnode_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestDIYSplitAccessLossExcludesImmediatelyAndReconcilesForFree(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	c := iplookup.DefaultConfig()
	c.LookupFeeTXB = "0"
	if err := f.store.SaveIPLookupSettings(ctx, f.user.ID, c, nil, nil); err != nil {
		t.Fatal(err)
	}
	config, err := f.service.SaveConfiguration(ctx, xnode.Configuration{Name: "Split hosts", Document: json.RawMessage(`{"inbounds":[{"tag":"paid","protocol":"vless","port":443,"settings":{"decryption":"none"},"streamSettings":{"network":"xhttp","xhttpSettings":{"path":"/shared"}}}],"outbounds":[{"tag":"direct","protocol":"freedom"},{"UserOutbound":{}}],"routing":{"rules":[{"UserRouting":{}}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	f.node.ConfigID = config.ID
	if _, err = f.service.SaveNode(ctx, f.node, xnode.NodeSettings{}); err != nil {
		t.Fatal(err)
	}
	up, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Upload", Address: "up.example", Port: 443, Enabled: true, Overrides: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	down, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Download", Address: "down.example", Port: 443, Enabled: true, Overrides: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.service.OwnedDIY(ctx, f.user.ID)
	if err != nil || len(view.SplitHosts) != 2 {
		t.Fatal(view.SplitHosts, err)
	}
	profiles := []xnode.SplitProfile{{ID: "one", UploadEndpointID: up.ID, DownloadEndpointID: down.ID, Mode: "packet-up"}}
	input := xnode.DIYWrite{Quote: view.Billing.Token, SettingsRevision: view.SettingsRevision, RoutingRevision: view.Routing.Plan.Revision, Choices: view.Routing.Plan.Choices, SplitProfiles: &profiles}
	result, err := f.service.SaveOwnedDIY(ctx, f.user.ID, "save-split", input)
	if err != nil || !result.Changed {
		t.Fatal(result, err)
	}
	// No agent is connected in this fixture. Desired host access remains valid.
	proxies, err := f.service.MemberProxies(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := f.service.ExportProxiesForUser(ctx, f.user.ID, proxies)
	if err != nil || len(exported) != 3 {
		t.Fatal("disconnection erased profile", len(exported), err)
	}
	out, err := exported[2].Outbound()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"inbounds": []any{}, "outbounds": []any{out}})
	if err != nil || !xnode.ValidateClientConfigurationDocument(raw).Valid {
		t.Fatal("split failed pinned parser", err)
	}
	before, err := f.store.XNodeDIY(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.service.DeleteEndpoint(ctx, down.ID); err != nil {
		t.Fatal(err)
	}
	proxies, err = f.service.MemberProxies(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	exported, err = f.service.ExportProxiesForUser(ctx, f.user.ID, proxies)
	if err != nil || len(exported) != 1 {
		t.Fatal("lost host remained exported", len(exported), err)
	}
	view, err = f.service.OwnedDIY(ctx, f.user.ID)
	if err != nil || len(view.SplitProfiles) != 0 || view.SettingsRevision != before.Revision {
		t.Fatal("read mutated stored references", view.SplitProfiles, err)
	}
	payload, _ := json.Marshal(map[string]string{"userId": f.user.ID})
	if err = f.service.ReconcileDIY(ctx, model.OutboxJob{Payload: string(payload)}); err != nil {
		t.Fatal(err)
	}
	after, err := f.store.XNodeDIY(ctx, f.user.ID)
	if err != nil || after.Revision != before.Revision+1 {
		t.Fatal("cleanup was not persisted", after.Revision, err)
	}
	// Cleanup does not create another user operation or debit.
	if _, found, _, err := f.store.ReplayXNodeDIY(ctx, f.user.ID, "save-split", "wrong"); err == nil || found {
		t.Fatal("original receipt lost fingerprint fence")
	}
}

package xnode_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestDIYRepairsPAYGInventoryWithPositiveSubGBBalance(t *testing.T) {
	f := controlFixture(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	config, err := f.service.SaveConfiguration(ctx, xnode.Configuration{Name: "PAYG XHTTP", Document: json.RawMessage(`{"inbounds":[{"tag":"paid","protocol":"vless","port":443,"settings":{"decryption":"none"},"streamSettings":{"network":"xhttp","xhttpSettings":{"path":"/shared"}}}],"outbounds":[{"tag":"direct","protocol":"freedom"},{"UserOutbound":{}}],"routing":{"rules":[{"UserRouting":{}}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	f.node.ConfigID = config.ID
	if _, err := f.service.SaveNode(ctx, f.node, xnode.NodeSettings{}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Upload", "Download"} {
		if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: name, Address: strings.ToLower(name) + ".example", Port: 443, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.store.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: f.group.ID, UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	balance, err := f.store.Balance(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AdjustBalance(ctx, f.user.ID, 1-balance.MinorInt64(), "small-topup", "fund", time.Now()); err != nil {
		t.Fatal(err)
	}
	view, err := f.service.OwnedDIY(ctx, f.user.ID)
	if err != nil || !view.InventoryPending {
		t.Fatal("DIY did not request missing PAYG inventory", view, err)
	}
	worker := outbox.NewWorker(f.store)
	for _, kind := range []string{xnode.AccessRepairJob, xnode.PAYGJob} {
		if err := worker.Register(kind, f.master); err != nil {
			t.Fatal(err)
		}
	}
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for view.InventoryPending {
		select {
		case <-ctx.Done():
			t.Fatal("DIY inventory never repaired", ctx.Err())
		case <-tick.C:
			if err := worker.Drain(ctx, 20); err != nil {
				t.Fatal(err)
			}
			view, err = f.service.OwnedDIY(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(view.Routing.Endpoints) != 2 || len(view.SplitHosts) != 2 {
		t.Fatal("PAYG missing from routing or XHTTP", view.Routing.Endpoints, view.SplitHosts)
	}
	for _, endpoint := range view.Routing.Endpoints {
		if !strings.HasPrefix(endpoint.ID, "payg-") {
			t.Fatal("DIY selected shared combo credentials", endpoint)
		}
	}
	payg, err := f.service.OwnedPAYG(ctx, f.user.ID)
	if err != nil || len(payg.Squads) != 1 || payg.Squads[0].Status != "insufficient_balance" {
		t.Fatal("DIY visibility bypassed transport funding", payg, err)
	}
	balance, err = f.store.Balance(ctx, f.user.ID)
	if err != nil || balance.MinorInt64() != 1 {
		t.Fatal("DIY read charged the member", balance, err)
	}
	// Exercise the same owner-scoped identifiers beyond the initial selectors.
	configIP := iplookup.DefaultConfig()
	configIP.LookupFeeTXB = "0"
	if err := f.store.SaveIPLookupSettings(ctx, f.user.ID, configIP, nil, nil); err != nil {
		t.Fatal(err)
	}
	view, err = f.service.OwnedDIY(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	profiles := []xnode.SplitProfile{{ID: "payg-split", UploadEndpointID: view.SplitHosts[0].ID, DownloadEndpointID: view.SplitHosts[0].DownloadIDs[0], Mode: "auto"}}
	input := xnode.DIYWrite{Quote: view.Billing.Token, SettingsRevision: view.SettingsRevision, RoutingRevision: view.Routing.Plan.Revision, Choices: view.Routing.Plan.Choices, SplitProfiles: &profiles}
	preview, err := f.service.PreviewOwnedDIY(ctx, f.user.ID, input)
	if err != nil || !preview.Valid {
		t.Fatal("PAYG split failed preview", preview, err)
	}
	if _, err := f.service.SaveOwnedDIY(ctx, f.user.ID, "payg-split", input); err != nil {
		t.Fatal("PAYG split failed save", err)
	}
	proxies, err := f.service.MemberProxies(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := f.service.ExportProxiesForUser(ctx, f.user.ID, proxies)
	if err != nil || len(exported) != 3 || exported[2].ID != "split:payg-split" || exported[2].PrincipalID != proxies[0].PrincipalID {
		t.Fatal("PAYG split lost owner credentials during export", exported, err)
	}
}

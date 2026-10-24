package xnode_test

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"testing"
	"time"
)

func provisionPAYGFixture(t *testing.T, f fixture) {
	t.Helper()
	ctx := context.Background()
	zero := 0
	if _, err := f.store.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: f.group.ID, UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100, StockLimit: &zero, ActivationRequired: true, ActivationCode: "old-gate"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]string{"userId": f.user.ID})
	if err := f.service.ReconcilePAYG(ctx, model.OutboxJob{Kind: xnode.PAYGJob, Payload: string(raw)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "PAYG host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
}

func TestPAYGSubscriptionRetainedWithoutWalletAndOwnerBanStillApplies(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	provisionPAYGFixture(t, f)
	proxies, err := f.service.MemberProxies(ctx, f.user.ID)
	if err != nil || len(proxies) != 1 || proxies[0].PrincipalID == 0 || proxies[0].PrincipalID == f.identity.ProxyUserID {
		t.Fatal("PAYG inventory gated by combo, stock, activation or funds", proxies, err)
	}
	metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
	if err != nil || !metadata.Active {
		t.Fatal("unfunded PAYG removed from subscription", metadata, err)
	}
	accounts, err := f.store.XNodePAYGAccounts(ctx, f.user.ID, f.node.ID)
	if err != nil || len(accounts) != 1 {
		t.Fatal(accounts, err)
	}
	internal, err := f.store.XNodeIdentity(ctx, accounts[0].ProxyUserID)
	if err != nil || internal.MemberOwnerID() != f.user.ID || len(internal.SubscriptionHash) != 0 {
		t.Fatal("PAYG principal lost owner isolation", internal, err)
	}
	if _, err = f.store.AdjustBalance(ctx, f.user.ID, 100, "fund", "fund", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = f.store.SetXNodeAccessDisabled(ctx, f.identity.ProxyUserID, true); err != nil {
		t.Fatal(err)
	}
	proxies, err = f.service.MemberProxies(ctx, f.user.ID)
	if err != nil || len(proxies) != 0 {
		t.Fatal("parent disable bypassed", proxies, err)
	}
	compiled, err := f.service.Compile(ctx, f.node.ID)
	if err != nil || len(compiled.Users) != 0 {
		t.Fatal("disabled owner retained PAYG admission", compiled.Users, err)
	}
}

func TestPAYGConversionDoesNotChargeOrReplacePaidCombo(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	provisionPAYGFixture(t, f)
	compiled, err := f.service.Compile(ctx, f.node.ID)
	if err != nil || len(compiled.Users) != 1 || compiled.Users[0].ID != int(f.identity.ProxyUserID) {
		t.Fatal("converted paid term changed principal", compiled.Users, err)
	}
	view, err := f.service.OwnedPAYG(ctx, f.user.ID)
	if err != nil || len(view.Squads) != 1 || view.Squads[0].Status != "combo_covered" {
		t.Fatal(view, err)
	}
	accounts, err := f.store.XNodePAYGAccounts(ctx, f.user.ID, "")
	if err != nil || accounts[0].PurchasedBytes != 0 {
		t.Fatal("conversion charged idle member", accounts, err)
	}
}

package xnode_test

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNewUnfundedMemberGetsPAYGSubscriptionThroughOnboardingQueue(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	if _, err := f.store.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: f.group.ID, UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "PAYG host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	user, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99104, FirstName: "Unfunded member"}, false)
	if err != nil {
		t.Fatal(err)
	}
	job := model.OutboxJob{Kind: xnode.PAYGJob, Payload: `{"userId":"` + user.ID + `"}`}
	if err := f.service.ReconcilePAYG(ctx, job); err != nil {
		t.Fatal(err)
	}
	if accounts, err := f.store.XNodePAYGAccounts(ctx, user.ID, ""); err != nil || len(accounts) != 0 {
		t.Fatal("incomplete onboarding granted PAYG", accounts, err)
	}
	if err := f.store.BeginAgreement(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	revision, ids, err := f.store.CurrentAgreementContract(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CompleteOnboardingRevision(ctx, user.ID, revision, ids, time.Now()); err != nil {
		t.Fatal(err)
	}
	worker := outbox.NewWorker(f.store)
	for _, kind := range []string{xnode.AccessRepairJob, xnode.PAYGJob, xnode.PAYGRecoveryJob} {
		if err := worker.Register(kind, f.master); err != nil {
			t.Fatal(err)
		}
	}
	if err := worker.Drain(ctx, 20); err != nil {
		t.Fatal(err)
	}
	accounts, err := f.store.XNodePAYGAccounts(ctx, user.ID, f.node.ID)
	if err != nil || len(accounts) != 1 || accounts[0].BalanceMinor != 0 || accounts[0].PurchasedBytes != 0 {
		t.Fatal("new member provisioning charged or missing", accounts, err)
	}
	view, err := f.service.OwnedPAYG(ctx, user.ID)
	if err != nil || len(view.Squads) != 1 || view.Squads[0].Status != "insufficient_balance" || view.Spent.MinorInt64() != 0 {
		t.Fatal("new member cannot browse PAYG or balance gate lost", view, err)
	}
	metadata, err := f.service.OwnedSubscription(ctx, user.ID)
	if err != nil || !metadata.Active || metadata.URL == "" {
		t.Fatal("new member subscription unavailable", metadata, err)
	}
	proxies, err := f.service.MemberProxies(ctx, user.ID)
	if err != nil || len(proxies) != 1 || proxies[0].PrincipalID != accounts[0].ProxyUserID {
		t.Fatal("unfunded PAYG links missing or wrong owner", proxies, err)
	}
	if purchases, err := f.store.ListPurchases(ctx, user.ID); err != nil || len(purchases) != 0 {
		t.Fatal("PAYG requires or creates a combo", purchases, err)
	}
	// Reconciliation is idempotent and preserves the new member's bearer URL.
	if err := f.service.ReconcilePAYG(ctx, job); err != nil {
		t.Fatal(err)
	}
	again, err := f.service.OwnedSubscription(ctx, user.ID)
	if err != nil || again.URL != metadata.URL {
		t.Fatal("PAYG retry rotated subscription", again, err)
	}
}

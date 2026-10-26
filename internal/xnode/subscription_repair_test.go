package xnode_test

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestMissingSubscriptionRepairRetainsIdentityCredentialsAndAccounting(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	u, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99109, FirstName: "PAYG recovery"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=?`, u.ID); err != nil {
		t.Fatal(err)
	}
	i, err := f.service.EnsureIdentity(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET subscription_cipher='',subscription_hash=NULL WHERE proxy_user_id=?`, i.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	job := model.OutboxJob{Kind: xnode.AccessRepairJob, Payload: `{"userId":"` + u.ID + `"}`}
	if err = f.service.RepairAccess(ctx, job); err != nil {
		t.Fatal(err)
	}
	after, err := f.service.IdentityForUser(ctx, u.ID)
	if err != nil || after.ProxyUserID != i.ProxyUserID || after.CredentialsCipher != i.CredentialsCipher || after.CurrentPeriodID != i.CurrentPeriodID {
		t.Fatal("repair replaced identity or accounting", after, err)
	}
	link, err := f.service.SubscriptionURL(after)
	if err != nil || link == "" {
		t.Fatal("subscription not repaired", err)
	}
	if err = f.service.RepairAccess(ctx, job); err != nil {
		t.Fatal(err)
	}
	again, err := f.service.IdentityForUser(ctx, u.ID)
	if err != nil || again.SubscriptionCipher != after.SubscriptionCipher {
		t.Fatal("retry rotated valid subscription", err)
	}
}

package xnode_test

import (
	"context"
	"testing"
	"time"
)

func TestNativeEntitlementNormalizesLastSquadWithoutLegacyIdentity(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `INSERT INTO user_disabled_squads VALUES(?,?)`, f.user.ID, f.group.ID); err != nil {
		t.Fatal(err)
	}
	purchase, err := f.store.DesiredEntitlement(ctx, f.user.ID, time.Now())
	if err != nil || purchase == nil {
		t.Fatal(purchase, err)
	}
	if err := f.service.ApplyPurchase(ctx, stringID(f.identity), purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil); err != nil {
		t.Fatal(err)
	}
	disabled, err := f.store.UserDisabledSquads(ctx, f.user.ID)
	if err != nil || len(disabled) != 0 {
		t.Fatal("native mutation left the last purchased squad disabled", disabled, err)
	}
	user, err := f.store.UserByID(ctx, f.user.ID)
	if err != nil || user.RemnaUserID != nil {
		t.Fatal("normalization manufactured a legacy link", user, err)
	}
}

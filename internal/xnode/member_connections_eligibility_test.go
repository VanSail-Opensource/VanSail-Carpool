package xnode_test

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"testing"
	"time"
)

func TestMemberGrantRequiresFundingAndEligibilityWithoutCharging(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	if _, err := f.service.GrantOwnedConnection(ctx, f.user.ID, "1.1.1.1"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("unfunded grant: %v", err)
	}
	if _, err := f.store.AdjustBalance(ctx, f.user.ID, 1, "lease-funding", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	before, err := f.store.Balance(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := f.service.GrantOwnedConnection(ctx, f.user.ID, "1.1.1.1")
	if err != nil || grant.LastObservedAt != nil || grant.ExpiresAt == nil {
		t.Fatal(grant, err)
	}
	again, err := f.service.GrantOwnedConnection(ctx, f.user.ID, "1.1.1.1")
	if err != nil || !again.ExpiresAt.Equal(*grant.ExpiresAt) {
		t.Fatal(again, err)
	}
	page, err := f.service.OwnedConnections(ctx, f.user.ID, "1.1.1.1", 1, 25)
	if err != nil || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
	after, err := f.store.Balance(ctx, f.user.ID)
	if err != nil || after.Minor != before.Minor {
		t.Fatal("grant charged balance", before, after, err)
	}
	if err = f.service.SaveAutomaticWhitelist(ctx, f.user.ID, xnode.AutomaticWhitelistConfig{Enabled: false, DurationHours: 24}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.GrantOwnedConnection(ctx, f.user.ID, "8.8.8.8"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("automation disabled: %v", err)
	}
	if err = f.service.SaveAutomaticWhitelist(ctx, f.user.ID, xnode.AutomaticWhitelistConfig{Enabled: true, DurationHours: 24}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='intro' WHERE id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.GrantOwnedConnection(ctx, f.user.ID, "8.8.8.8"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("onboarding: %v", err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.GrantOwnedConnection(ctx, f.user.ID, "8.8.8.8"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("account disabled: %v", err)
	}
}

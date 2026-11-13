package app

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestNativeAdminBanPreservesOwnerPeriodAndRejectsStaleRelease(t *testing.T) {
	f := newNativePurchaseFixture(t)
	ctx := context.Background()
	actor, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: f.user.TelegramID + 2}, true)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: f.user.TelegramID + 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	otherIdentity, err := f.provider.native.EnsureIdentity(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET proxy_user_id=$1 WHERE id=$2`, strconv.FormatInt(otherIdentity.ProxyUserID, 10), f.user.ID); err != nil {
		t.Fatal(err)
	}
	identity, err := f.provider.native.IdentityForUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	periodID := identity.CurrentPeriodID
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=7 WHERE id=$1`, periodID); err != nil {
		t.Fatal(err)
	}
	workflow := admin.NewUserWorkflows(f.store, f.provider)
	ban, err := workflow.TemporaryBan(ctx, actor.ID, f.user.ID, "ban-first", "investigation", 10)
	if err != nil {
		t.Fatal(err)
	}
	runNativeBanOperation(t, f, ban.ID)
	identity, err = f.provider.native.IdentityForUser(ctx, f.user.ID)
	if err != nil || !identity.AccessDisabled {
		t.Fatalf("native owner was not restricted: %+v, %v", identity, err)
	}
	unban, err := workflow.TemporaryUnban(ctx, actor.ID, f.user.ID, "unban-first", "investigation complete")
	if err != nil {
		t.Fatal(err)
	}
	runNativeBanOperation(t, f, unban.ID)
	identity, err = f.provider.native.IdentityForUser(ctx, f.user.ID)
	period, periodErr := f.store.XNodePeriod(ctx, periodID)
	if err != nil || periodErr != nil || identity.AccessDisabled || identity.CurrentPeriodID != periodID || period.Status != "open" || period.UsedBytes != 7 {
		t.Fatalf("restoration reset accounting: %+v %+v %v %v", identity, period, err, periodErr)
	}
	second, err := workflow.TemporaryBan(ctx, actor.ID, f.user.ID, "ban-second", "another investigation", 10)
	if err != nil {
		t.Fatal(err)
	}
	runNativeBanOperation(t, f, second.ID)
	if err := f.provider.SetAdminUserDisabled(ctx, f.user, unban.ID, false); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("stale release error = %v", err)
	}
	identity, err = f.provider.native.IdentityForUser(ctx, f.user.ID)
	otherAfter, otherErr := f.provider.native.IdentityForUser(ctx, other.ID)
	if err != nil || otherErr != nil || !identity.AccessDisabled || otherAfter.AccessDisabled {
		t.Fatalf("owner isolation after repeat ban: %+v %+v %v %v", identity, otherAfter, err, otherErr)
	}
}

func runNativeBanOperation(t *testing.T, f nativePurchaseFixture, id string) {
	t.Helper()
	operation, err := f.store.ProviderOperationByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.NewUserProviderWorker(f.store, f.provider).HandleProviderOperation(context.Background(), operation, model.OutboxJob{Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	operation, err = f.store.ProviderOperationByID(context.Background(), id)
	if err != nil || operation.Receipt.Status != "succeeded" {
		t.Fatalf("native restriction operation = %+v, %v", operation, err)
	}
}

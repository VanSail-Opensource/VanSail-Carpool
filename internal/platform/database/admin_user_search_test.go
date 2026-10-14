package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestAdminUserSearchCombinesFacetsAndBindsCursors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Now().UTC()
	comboA := saveTestCombo(t, store, "admin-filter-a", 100, 30)
	comboB := saveTestCombo(t, store, "admin-filter-b", 100, 30)
	squad := saveTestSquad(t, store, "a0a0a0a0-1111-4111-8111-111111111111", 10, true)
	activeA, _ := createAdminWorkflowPurchase(t, store, 48_001, comboA, now, squad.RemnaSquadUUID)
	activeB, _ := createAdminWorkflowPurchase(t, store, 48_002, comboB, now)
	inactive, expired := createAdminWorkflowPurchase(t, store, 48_003, comboB, now)
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='expired' WHERE id=?`, expired.ID); err != nil {
		t.Fatal(err)
	}

	items, _, err := store.ListAdminUsersPage(ctx, "", AdminUserSearchFilter{State: "active", ComboIDs: []string{comboA.ID}, SquadUUIDs: []string{squad.RemnaSquadUUID}, Match: "and"}, 25)
	if err != nil || len(items) != 1 || items[0].User.ID != activeA.ID {
		t.Fatalf("AND facet result = (%+v, %v)", items, err)
	}
	items, _, err = store.ListAdminUsersPage(ctx, "", AdminUserSearchFilter{State: "non_active", ComboIDs: []string{comboB.ID}, Match: "and"}, 25)
	if err != nil || len(items) != 1 || items[0].User.ID != inactive.ID {
		t.Fatalf("non-active combo result = (%+v, %v)", items, err)
	}
	items, _, err = store.ListAdminUsersPage(ctx, "", AdminUserSearchFilter{Search: "no such member", State: "active", ComboIDs: []string{comboA.ID}, Match: "or"}, 25)
	if err != nil || len(items) != 0 {
		t.Fatalf("free-text narrowing result = (%+v, %v)", items, err)
	}

	filter := AdminUserSearchFilter{State: "active", ComboIDs: []string{comboB.ID, comboA.ID}, Match: "and"}
	items, cursor, err := store.ListAdminUsersPage(ctx, "", filter, 1)
	if err != nil || len(items) != 1 || cursor == nil {
		t.Fatalf("first cursor page = (%+v, %v, %v)", items, cursor, err)
	}
	firstID := items[0].User.ID
	items, _, err = store.ListAdminUsersPage(ctx, *cursor, AdminUserSearchFilter{State: "active", ComboIDs: []string{comboA.ID, comboB.ID}, Match: "and"}, 1)
	if err != nil || len(items) != 1 || items[0].User.ID == firstID || (items[0].User.ID != activeA.ID && items[0].User.ID != activeB.ID) {
		t.Fatalf("normalized cursor page = (%+v, %v)", items, err)
	}
	if _, _, err = store.ListAdminUsersPage(ctx, *cursor, AdminUserSearchFilter{State: "active", ComboIDs: []string{comboA.ID}, Match: "and"}, 1); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("changed-filter cursor error = %v, want ErrInvalidCursor", err)
	}
}

func TestSuccessfulAffiliateReferralsExcludesPendingMembers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	inviters := createTestUser(t, store, 48_100)
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 48_101, 48_100, time.Now().UTC()); err != nil || !accepted {
		t.Fatalf("AcceptAffiliateReferral(settled) = (%v, %v)", accepted, err)
	}
	settled, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 48_101, FirstName: "Settled", Username: "settled-affiliate"}, false)
	if err != nil {
		t.Fatal(err)
	}
	order := createTestPaymentOrder(t, store, settled.ID, "ezpay", 500, time.Now().UTC())
	if _, changed, err := store.SettlePayment(ctx, "ezpay", "admin-affiliate-settled", "hash", order.ID, "trade", "", time.Now().UTC()); err != nil || !changed {
		t.Fatalf("SettlePayment() = (%v, %v)", changed, err)
	}
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 48_102, 48_100, time.Now().UTC()); err != nil || !accepted {
		t.Fatalf("AcceptAffiliateReferral(pending) = (%v, %v)", accepted, err)
	}
	if _, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 48_102, FirstName: "Pending", Username: "pending-affiliate"}, false); err != nil {
		t.Fatal(err)
	}
	page, err := store.SuccessfulAffiliateReferrals(ctx, inviters.ID, 50)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Status != "successful" {
		t.Fatalf("SuccessfulAffiliateReferrals() = (%+v, %v)", page, err)
	}
}

package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestProductStatisticsExcludeAdministratorsFromMemberMetrics(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	member := createTestUser(t, store, 71_001)
	admin, created, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 71_002, FirstName: "Admin"}, true)
	if err != nil || !created {
		t.Fatalf("create administrator = (%+v, %v, created=%v)", admin, err, created)
	}
	for _, identity := range []struct {
		userID, remoteID string
	}{
		{userID: member.ID, remoteID: "member-proxy-id"},
		{userID: admin.ID, remoteID: "admin-proxy-id"},
	} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE users SET proxy_user_id=? WHERE id=?`, identity.remoteID, identity.userID); err != nil {
			t.Fatalf("set Proxy identity: %v", err)
		}
	}
	memberSquad := saveTestSquad(t, store, "member-squad", 0, true)
	adminSquad := saveTestSquad(t, store, "admin-squad", 0, true)
	adminAddon := saveTestSquad(t, store, "admin-addon", 50, true)
	memberCombo := saveTestCombo(t, store, "Member combo", 100, 30, memberSquad.ID)
	adminCombo := saveTestCombo(t, store, "Admin combo", 900, 30, adminSquad.ID)
	for index, seed := range []struct {
		userID string
		amount int64
	}{
		{userID: member.ID, amount: 100},
		{userID: admin.ID, amount: 950},
	} {
		if _, err := store.AdjustBalance(ctx, seed.userID, seed.amount, fmt.Sprintf("statistics_seed:%d", index), "test", now); err != nil {
			t.Fatalf("seed balance: %v", err)
		}
	}
	memberPurchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: member.ID, ComboID: memberCombo.ID, IdempotencyKey: "member-purchase"}, now)
	if err != nil {
		t.Fatalf("create member purchase: %v", err)
	}
	if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: admin.ID, ComboID: adminCombo.ID,
		AddonSquadIDs: []string{adminAddon.ID}, IdempotencyKey: "admin-purchase"}, now); err != nil {
		t.Fatalf("create admin purchase: %v", err)
	}

	statistics, err := store.ProductDatabaseStatistics(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ProductDatabaseStatistics(): %v", err)
	}
	if statistics.NewUserConversion != 100 || statistics.AverageSpend.Minor != "100" ||
		statistics.SpendMinimum.Minor != "100" || statistics.SpendMaximum.Minor != "100" {
		t.Fatalf("member conversion/spend = %+v", statistics)
	}
	if shareTotal(statistics.SubscriptionStates) != 1 || len(statistics.ComboShares) != 1 ||
		statistics.ComboShares[0].ID != memberCombo.ID || statistics.ComboShares[0].Value != 1 || statistics.AverageOptionalSquads != 0 {
		t.Fatalf("member state/catalog metrics = %+v", statistics)
	}
	if len(statistics.SquadByCombo) != 1 || statistics.SquadByCombo[0].ID != memberCombo.ID ||
		len(statistics.ComboBySquad) != 1 || statistics.ComboBySquad[0].ID != memberSquad.GroupID {
		t.Fatalf("member distributions = (%+v, %+v)", statistics.SquadByCombo, statistics.ComboBySquad)
	}
	members, err := store.ActiveMemberUsageForStatistics(ctx, now.Add(time.Minute))
	if err != nil || len(members) != 1 || members[0].Purchase.ID != memberPurchase.ID || members[0].RemoteUserID == "" {
		t.Fatalf("ActiveMemberUsageForStatistics() = (%+v, %v)", members, err)
	}
}

func TestProductStatisticsUseEffectiveSquadOverrides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 13, 0, 0, 0, time.UTC)
	included := saveTestSquad(t, store, "11111111-1111-4111-8111-111111111111", 0, true)
	staleAddon := saveTestSquad(t, store, "22222222-2222-4222-8222-222222222222", 25, true)
	fullOverride := saveTestSquad(t, store, "33333333-3333-4333-8333-333333333333", 0, true)
	addonOverride := saveTestSquad(t, store, "44444444-4444-4444-8444-444444444444", 0, true)
	combo := saveTestCombo(t, store, "Override combo", 100, 30, included.ID)
	first, firstPurchase := createAdminWorkflowPurchase(t, store, 72_001, combo, now, staleAddon.GroupID)
	second, secondPurchase := createAdminWorkflowPurchase(t, store, 72_002, combo, now, staleAddon.GroupID)
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_squad_uuids=? WHERE id=?`,
		`["`+fullOverride.GroupID+`"]`, firstPurchase.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_addon_squad_uuids=? WHERE id=?`,
		`["`+addonOverride.GroupID+`"]`, secondPurchase.ID); err != nil {
		t.Fatal(err)
	}
	statistics, err := store.ProductDatabaseStatistics(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ProductDatabaseStatistics(): %v", err)
	}
	if statistics.AverageOptionalSquads != 1 {
		t.Fatalf("average optional squads = %v, want 1", statistics.AverageOptionalSquads)
	}
	wantSquads := map[string]bool{included.GroupID: true, fullOverride.GroupID: true, addonOverride.GroupID: true}
	if len(statistics.ComboBySquad) != len(wantSquads) {
		t.Fatalf("combo-by-squad = %+v", statistics.ComboBySquad)
	}
	for _, group := range statistics.ComboBySquad {
		if !wantSquads[group.ID] || group.ID == staleAddon.GroupID {
			t.Fatalf("unexpected effective squad distribution for users %s/%s: %+v", first.ID, second.ID, statistics.ComboBySquad)
		}
	}
}

func TestProductStatisticsCountDeduplicatedRawGroupMessages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 14, 0, 0, 0, time.UTC)
	for _, messageID := range []int64{1, 1, 2} {
		if err := store.RecordGroupMessageFact(ctx, -100, messageID, "2026-08-18", now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO activity_daily_rollups(local_date,group_message_count,updated_at)
		VALUES('2026-08-17',3,?)`, stamp(now)); err != nil {
		t.Fatal(err)
	}
	statistics, err := store.ProductDatabaseStatistics(ctx, now)
	if err != nil || statistics.GroupMessagesTotal != 5 {
		t.Fatalf("group messages total = %d, %v; want 5", statistics.GroupMessagesTotal, err)
	}
}

func TestProductStatisticsExcludeFutureActiveRowsAndIgnoredPaymentStates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 16, 0, 0, 0, time.UTC)
	user := createTestUser(t, store, 72_003)
	combo := saveTestCombo(t, store, "future-active", 100, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "future-active", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "future-active"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_from=?,valid_until=? WHERE id=?`,
		stamp(now.Add(time.Hour)), stamp(now.Add(31*24*time.Hour)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	for index, status := range []string{"failed", "refunded"} {
		if _, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{UserID: user.ID, Provider: "bepusdt", Status: status,
			TXBMinor: int64(100 + index), PayableAmount: "1", PayableCurrency: "USDT", RateSnapshot: "1", ExpiresAt: now.Add(time.Hour)}); err != nil {
			t.Fatalf("CreatePaymentOrder(%s): %v", status, err)
		}
	}
	statistics, err := store.ProductDatabaseStatistics(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(statistics.ComboShares) != 0 || shareValue(statistics.SubscriptionStates, "no_active") != 1 {
		t.Fatalf("future active metrics = states %+v, combos %+v", statistics.SubscriptionStates, statistics.ComboShares)
	}
	if len(statistics.PaymentStatuses) != 0 {
		t.Fatalf("ignored payment shares = %+v", statistics.PaymentStatuses)
	}
}

func shareTotal(shares []model.NamedShare) float64 {
	var total float64
	for _, share := range shares {
		total += share.Value
	}
	return total
}

func shareValue(shares []model.NamedShare, id string) float64 {
	for _, share := range shares {
		if share.ID == id {
			return share.Value
		}
	}
	return 0
}

package catalog

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestAutomaticRenewalEnablesAndChargesRepricedHeldSquad(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "renewal-stock.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	store := database.NewStore(db)
	service := newCatalogServiceForTest(store, renewalTestRemote())
	now := service.now()
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 51_010, FirstName: "Renewal member"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 150_000, "held-renewal-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	addon, err := store.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: "retained",
		Name: "Retained", UpstreamPresent: true, Visible: true, PriceTXBMinor: 10_000})
	if err != nil {
		t.Fatal(err)
	}
	combo, err := store.SaveCombo(ctx, database.ComboInput{Name: "Renewal combo", PriceTXBMinor: 31_900,
		ValidityDays: 30, TrafficLimitBytes: 500_000_000_000, ResetStrategy: "MONTH_ROLLING", Active: true,
		SquadProductIDs: []string{"core-squad"}})
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: combo.ID,
		AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "held-renewal-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, false, now); err != nil {
		t.Fatal(err)
	}
	zero := 0
	if _, err := store.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: addon.ID,
		Name: addon.Name, UpstreamPresent: true, Visible: true, PriceTXBMinor: 49_900, StockLimit: &zero}); err != nil {
		t.Fatal(err)
	}
	remote := renewalTestRemote()
	remote.squads = append(remote.squads, RemoteSquad{UUID: addon.ID, Name: addon.Name})
	remote.accessible[addon.ID] = []string{"node-1"}
	service = newCatalogServiceForTest(store, remote)
	status, err := service.AutomaticRenewal(ctx, user, source.ID)
	if err != nil || !status.CanEnable || status.IneligibleReason != nil || status.GrossPrice.Minor != "81800" || status.NetPrice.Minor != "81800" {
		t.Fatalf("AutomaticRenewal() = (%+v, %v), want eligible 818.00 TXB", status, err)
	}
	status, err = service.SetAutomaticRenewal(ctx, user, source.ID, true)
	if err != nil || !status.Enabled || status.NetPrice.Minor != "81800" {
		t.Fatalf("SetAutomaticRenewal() = (%+v, %v), want enabled 818.00 TXB", status, err)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	if err := service.ProcessDueAutoRenewals(ctx, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	// Re-enter the complete scheduler path: the successor must not be charged twice.
	if err := service.ProcessDueAutoRenewals(ctx, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	var successorID string
	var successors int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(MAX(id),'') FROM purchases WHERE auto_renew_source_purchase_id=?`, source.ID).Scan(&successors, &successorID); err != nil {
		t.Fatal(err)
	}
	if successors != 1 {
		t.Fatalf("successors = %d, want 1", successors)
	}
	renewed, err := store.PurchaseByID(ctx, successorID)
	if err != nil || renewed.PriceTXBMinor != 81_800 || renewed.CoreGrossTXBMinor != 31_900 || !renewed.AutoRenewEnabled {
		t.Fatalf("renewed purchase = (%+v, %v)", renewed, err)
	}
	if balance, err := store.Balance(ctx, user.ID); err != nil || balance.Minor != "26300" {
		t.Fatalf("Balance() = (%+v, %v), want one current-price debit", balance, err)
	}
	var debitCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='automatic_renewal'`, user.ID).Scan(&debitCount); err != nil || debitCount != 1 {
		t.Fatalf("renewal debits = %d, %v", debitCount, err)
	}
}

package app

import (
	"context"
	"net/url"

	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type nativePurchaseFixture struct {
	store    *database.Store
	provider proxyProvider
	user     model.User
	purchase model.Purchase
	remoteID string
}

func newNativePurchaseFixture(t *testing.T) nativePurchaseFixture {
	t.Helper()
	ctx := context.Background()
	db, err := testdatabase.Open(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	store := database.NewStore(db)
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	public, _ := url.Parse("https://panel.example")
	native := xnode.NewService(store, nil, vault, public, nil)
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 89020, FirstName: "Native"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	user, err = store.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := store.AdjustBalance(ctx, user.ID, 10000, "seed", "fixture", now); err != nil {
		t.Fatal(err)
	}
	combo, err := store.SaveCombo(ctx, database.ComboInput{Name: "Native", ValidityDays: 30, PriceTXBMinor: 300, TrafficLimitBytes: 1000, ResetStrategy: "DAY", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "native-purchase"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	purchase, err = store.PurchaseByID(ctx, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := native.EnsureIdentity(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	remoteID := strconv.FormatInt(identity.ProxyUserID, 10)
	if err := native.ApplyPurchase(ctx, remoteID, purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil); err != nil {
		t.Fatal(err)
	}
	return nativePurchaseFixture{store: store, provider: proxyProvider{native: native, store: store}, user: user, purchase: purchase, remoteID: remoteID}
}

func (f nativePurchaseFixture) resetReceipt(t *testing.T) model.OperationReceipt {
	t.Helper()
	ctx, now := context.Background(), time.Now().UTC()
	// Seed a paid receipt from the retired release; there is no new reset API.
	operation, _, err := f.store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: f.user.ID, OwnerUserID: f.user.ID, Kind: "purchase_traffic_reset", IdempotencyKey: "reset", RequestFingerprint: "legacy-reset-obligation", Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: f.purchase.ID}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var balance int64
	if err = tx.QueryRowContext(ctx, `UPDATE balances SET txb_minor=txb_minor-10 WHERE user_id=$1 RETURNING txb_minor`, f.user.ID).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,note,created_at) VALUES($1,$2,-10,$3,'traffic_reset_debit',$4,'historical reset',$5)`, operation.Receipt.ID+":debit", f.user.ID, balance, operation.Receipt.ID, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return operation.Receipt
}

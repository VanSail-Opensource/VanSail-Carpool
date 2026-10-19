package app

import (
	"context"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
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
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "purchase.db"))
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
	if _, err := db.ExecContext(ctx, `UPDATE users SET username='nativepurchase',onboarding_state='complete' WHERE id=?`, user.ID); err != nil {
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
	service := purchaseops.NewService(f.store, f.provider)
	receipt, err := service.ResetTraffic(context.Background(), f.user.ID, f.purchase.ID, "reset")
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

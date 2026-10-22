package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"testing"
	"time"
)

func TestInactiveMemberTagsStayReservedAcrossNumericNamespaces(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	first := json.RawMessage(`[{"tag":"peer-bob","protocol":"socks","settings":{"servers":[{"address":"1.1.1.1","port":1080}]}}]`)
	if _, err := f.service.SaveOwnedOutbounds(ctx, f.user.ID, f.node.ID, 0, first); err != nil {
		t.Fatal(err)
	}
	purchases, err := f.store.ListPurchases(ctx, f.user.ID)
	if err != nil || len(purchases) != 1 {
		t.Fatal("fixture purchase missing")
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET status='expired' WHERE user_id=?`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	user, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99102, FirstName: "Second"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE users SET username='bob-alice',onboarding_state='complete' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	user, err = f.store.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := f.service.EnsureIdentity(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	purchase, err := f.store.CreatePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: purchases[0].ComboID, IdempotencyKey: "second"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = f.service.ApplyPurchase(ctx, stringID(identity), purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil); err != nil {
		t.Fatal(err)
	}
	second := json.RawMessage(`[{"tag":"peer","protocol":"socks","settings":{"servers":[{"address":"1.1.1.1","port":1080}]}}]`)
	if _, err = f.service.SaveOwnedOutbounds(ctx, user.ID, f.node.ID, 0, second); err != nil {
		t.Fatalf("independent numeric namespace rejected: %v", err)
	}
	config, err := f.service.ResolveConfiguration(ctx, f.node.ConfigID)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if err = json.Unmarshal(config.Document, &native); err != nil {
		t.Fatal(err)
	}
	native["outbounds"] = append(native["outbounds"].([]any), map[string]any{"tag": "peer-bob-" + stringID(f.identity), "protocol": "freedom"})
	config.Document, err = json.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.SaveConfiguration(ctx, config); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("administrator took an inactive member's numeric tag: %v", err)
	}
}

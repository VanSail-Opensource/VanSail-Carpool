package xnode_test

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

type fixtureSettings struct{ raw string }

func (f *fixtureSettings) Optional(context.Context, string) (string, error) { return f.raw, nil }
func (f *fixtureSettings) PutXNodeGlobal(_ context.Context, _ string, raw string) error {
	f.raw = raw
	return nil
}

type fixture struct {
	service  *xnode.Service
	master   *xnode.Master
	store    *database.Store
	node     xnode.Node
	group    xnode.Group
	user     model.User
	identity xnode.Identity
}

func controlFixture(t *testing.T, paid bool) fixture {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "panel.db"))
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
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "fixture-validation", Capacity: 8})
	if err != nil {
		t.Fatal(err)
	}
	lifetime, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	if err = queue.Start(lifetime); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queue.Shutdown(context.Background()) })
	public, _ := url.Parse("https://panel.example")
	service := xnode.NewService(store, &fixtureSettings{}, vault, public, queue)
	master := xnode.NewMaster(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err = master.Start(lifetime); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		child, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		if err := master.Shutdown(child); err != nil {
			t.Error(err)
		}
	})
	config, err := service.SaveConfiguration(ctx, xnode.Configuration{Name: "Hosted fixture", Document: json.RawMessage(`{"inbounds":[{"tag":"paid","protocol":"vless","port":443,"settings":{"decryption":"none"}}],"outbounds":[{"tag":"direct","protocol":"freedom"},{"UserOutbound":{}}],"routing":{"rules":[{"UserRouting":{}}]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	node, err := service.SaveNode(ctx, xnode.Node{Name: "Hosted fixture", Enabled: true, Multiplier: 1, ConfigID: config.ID}, xnode.NodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	group, err := service.SaveGroup(ctx, xnode.Group{Name: "Paid group", Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: node.ID, InboundTag: "paid"}}})
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 99101, FirstName: "Fixture"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE users SET username='alice',onboarding_state='complete' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	user, err = store.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := service.EnsureIdentity(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		combo, err := store.SaveCombo(ctx, database.ComboInput{Name: "Paid", ValidityDays: 30, TrafficLimitBytes: 1 << 30, ResetStrategy: "NO_RESET", Active: true, SquadProductIDs: []string{group.ID}})
		if err != nil {
			t.Fatal(err)
		}
		purchase, err := store.CreatePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "fixture"}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if err = service.ApplyPurchase(ctx, stringID(identity), purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil); err != nil {
			t.Fatal(err)
		}
	}
	return fixture{service: service, master: master, store: store, node: node, group: group, user: user, identity: identity}
}

package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func TestRemnawaveAdapterMethodsEnterQueueBeforeClientCreation(t *testing.T) {
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "remna-test", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	var factoryCalls atomic.Int32
	adapter := remnaAdapter{
		queue: queue,
		clientFactory: func(context.Context) (remnaClient, error) {
			factoryCalls.Add(1)
			return nil, errors.New("client factory must not run")
		},
	}
	createInput := accounts.RemoteCreateUser{
		Username: "tester", Status: "ACTIVE", ExpireAt: time.Now().Add(time.Hour),
		TelegramID: 42, TrafficLimitStrategy: "NO_RESET",
	}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "find username", call: func() error { _, _, err := adapter.FindUserByUsername(context.Background(), "tester"); return err }},
		{name: "find Telegram id", call: func() error { _, _, err := adapter.FindUserByTelegramID(context.Background(), 42); return err }},
		{name: "find remote id", call: func() error { _, _, err := adapter.FindUserByID(context.Background(), "1"); return err }},
		{name: "create user", call: func() error { _, err := adapter.CreateUser(context.Background(), createInput); return err }},
		{name: "dashboard", call: func() error { _, err := adapter.Dashboard(context.Background(), "1"); return err }},
		{name: "member subscription", call: func() error {
			_, err := adapter.LoadMemberSubscription(context.Background(), "1", []string{"owned"}, true)
			return err
		}},
		{name: "revoke subscription", call: func() error { _, err := adapter.RevokeSubscription(context.Background(), "1"); return err }},
		{name: "apply entitlement", call: func() error {
			return adapter.ApplyEntitlement(context.Background(), "1", 10, "NO_RESET", []string{"squad"}, time.Now().Add(time.Hour))
		}},
		{name: "reset traffic", call: func() error { return adapter.ResetTraffic(context.Background(), "1") }},
		{name: "remove entitlement", call: func() error { return adapter.RemoveEntitlement(context.Background(), "1") }},
		{name: "quiesce rollover", call: func() error { return adapter.QuiesceForRollover(context.Background(), "1") }},
		{name: "list notification traffic", call: func() error {
			_, _, _, err := adapter.ListNotificationUsers(context.Background(), "", 1000)
			return err
		}},
		{name: "list admin squads", call: func() error { _, err := adapter.ListInternalSquads(context.Background()); return err }},
		{name: "list catalog squads", call: func() error { _, err := adapter.ListCatalogSquads(context.Background()); return err }},
		{name: "list catalog nodes", call: func() error { _, err := adapter.ListCatalogNodes(context.Background()); return err }},
		{name: "accessible catalog nodes", call: func() error { _, err := adapter.AccessibleCatalogNodeUUIDs(context.Background(), "squad"); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalls.Store(0)
			if err := test.call(); !errors.Is(err, upstreamqueue.ErrNotRunning) {
				t.Fatalf("adapter error = %v, want ErrNotRunning", err)
			}
			if calls := factoryCalls.Load(); calls != 0 {
				t.Fatalf("client factory calls = %d, want 0", calls)
			}
		})
	}
}

func TestEmbyAdapterMethodsEnterQueueBeforeClientCreation(t *testing.T) {
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "emby-test", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	var factoryCalls atomic.Int32
	adapter := embyAdapter{
		queue: queue,
		clientFactory: func(context.Context) (embyClient, error) {
			factoryCalls.Add(1)
			return nil, errors.New("client factory must not run")
		},
	}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "find user", call: func() error { _, _, err := adapter.FindUserByName(context.Background(), "tester"); return err }},
		{name: "create user", call: func() error { _, err := adapter.CreateUser(context.Background(), "tester"); return err }},
		{name: "get user", call: func() error { _, err := adapter.GetUser(context.Background(), "user"); return err }},
		{name: "set password", call: func() error { return adapter.SetPassword(context.Background(), "user", []byte("old"), []byte("new")) }},
		{name: "update policy", call: func() error { return adapter.UpdatePolicy(context.Background(), "user", domain.Policy{}) }},
		{name: "list folders", call: func() error { _, err := adapter.ListSelectableFolders(context.Background()); return err }},
		{name: "list ratings", call: func() error { _, err := adapter.ListParentalRatings(context.Background()); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalls.Store(0)
			if err := test.call(); !errors.Is(err, upstreamqueue.ErrNotRunning) {
				t.Fatalf("adapter error = %v, want ErrNotRunning", err)
			}
			if calls := factoryCalls.Load(); calls != 0 {
				t.Fatalf("client factory calls = %d, want 0", calls)
			}
		})
	}
}

func TestTelegramAdapterMethodsEnterQueue(t *testing.T) {
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "telegram-test", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	adapter := &queuedTelegram{queue: queue}
	tests := []func() error{
		func() error { _, err := adapter.GetMe(context.Background()); return err },
		func() error { return adapter.SendMarkdownV2Message(context.Background(), 42, 0, "hello") },
		func() error { _, err := adapter.GetStarTransactions(context.Background(), 0, 100); return err },
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, upstreamqueue.ErrNotRunning) {
			t.Fatalf("adapter error = %v, want ErrNotRunning", err)
		}
	}
}

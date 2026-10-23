package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

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

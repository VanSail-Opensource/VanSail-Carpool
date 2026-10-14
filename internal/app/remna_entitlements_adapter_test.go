package app

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type entitlementUpdateClient struct {
	remnaClient
	updates []remnawave.UpdateUserRequest
}

func (c *entitlementUpdateClient) UpdateUser(_ context.Context, input remnawave.UpdateUserRequest) (*remnawave.User, error) {
	c.updates = append(c.updates, input)
	return &remnawave.User{}, nil
}

func TestRemnaEntitlementAdapterSyncsActiveAndDisabledExpiry(t *testing.T) {
	t.Parallel()

	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "remna-expiry-test", Capacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = queue.Shutdown(context.Background()) }()

	client := &entitlementUpdateClient{}
	adapter := remnaAdapter{
		queue: queue,
		clientFactory: func(context.Context) (remnaClient, error) {
			return client, nil
		},
	}
	activeExpiry := time.Date(2027, 8, 14, 20, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if err := adapter.ApplyEntitlement(context.Background(), "7", 1024, "MONTH", []string{"squad"}, activeExpiry); err != nil {
		t.Fatalf("ApplyEntitlement(): %v", err)
	}
	if err := adapter.RemoveEntitlement(context.Background(), "7"); err != nil {
		t.Fatalf("RemoveEntitlement(): %v", err)
	}
	if len(client.updates) != 2 {
		t.Fatalf("UpdateUser calls = %d, want 2", len(client.updates))
	}
	active := client.updates[0]
	if active.ExpireAt == nil || !active.ExpireAt.Equal(activeExpiry.UTC()) {
		t.Fatalf("active expiry = %v, want %s", active.ExpireAt, activeExpiry.UTC())
	}
	if active.Status == nil || *active.Status != remnawave.UserStatusActive {
		t.Fatalf("active status = %v, want ACTIVE", active.Status)
	}
	disabled := client.updates[1]
	if disabled.ExpireAt == nil || !disabled.ExpireAt.Equal(time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)) {
		t.Fatalf("disabled expiry = %v, want 2099-12-31T23:59:59Z", disabled.ExpireAt)
	}
	if disabled.Status == nil || *disabled.Status != remnawave.UserStatusDisabled {
		t.Fatalf("disabled status = %v, want DISABLED", disabled.Status)
	}
}

package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnodemigration"
)

type sourcePageClient struct {
	remnaClient
	users []remnawave.User
	loop  bool
	calls int
}

func (c *sourcePageClient) ListUsersStream(_ context.Context, cursor string, limit int) ([]remnawave.User, *string, bool, error) {
	c.calls++
	if limit != 200 {
		return nil, nil, false, xnode.ErrInvalid
	}
	if c.loop {
		next := "first"
		if cursor == "first" {
			next = "second"
		}
		return nil, &next, true, nil
	}
	return c.users, nil, false, nil
}

func TestMigrationPreviewReportsUnmappedUsersWithoutTheirSecrets(t *testing.T) {
	client := &sourcePageClient{users: []remnawave.User{
		{ID: 41, Username: "linked", Status: remnawave.UserStatusActive, ExpireAt: time.Now().Add(time.Hour)},
		{ID: 42, Username: "unmapped", Status: remnawave.UserStatusActive, ExpireAt: time.Now().Add(time.Hour), VLESSUUID: "private-uuid", TrojanPassword: "private-trojan", ShadowsocksPassword: "private-ss"},
		{ID: 43, Username: "expired", Status: remnawave.UserStatusActive, ExpireAt: time.Now().Add(-time.Hour)},
	}}
	adapter := newRolloverStatsAdapter(t, &rolloverStatsClient{}, newNodeMultiplierCache())
	adapter.clientFactory = func(context.Context) (remnaClient, error) { return client, nil }
	snapshot := xnodemigration.Snapshot{}
	if err := (migrationSource{}).captureUnmatched(context.Background(), adapter, &snapshot, map[int64]bool{41: true}); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Accounts) != 1 || snapshot.Accounts[0].LegacyID != 42 || snapshot.Accounts[0].UUID != "" || snapshot.Accounts[0].Trojan != "" || snapshot.Accounts[0].Chacha != "" {
		t.Fatalf("unsafe unmatched inventory=%+v", snapshot.Accounts)
	}
}

func TestMigrationSourceRejectsMultiCursorLoops(t *testing.T) {
	client := &sourcePageClient{loop: true}
	adapter := newRolloverStatsAdapter(t, &rolloverStatsClient{}, newNodeMultiplierCache())
	adapter.clientFactory = func(context.Context) (remnaClient, error) { return client, nil }
	err := (migrationSource{}).visitAccounts(context.Background(), adapter, func(remnawave.User) error { return nil })
	if !errors.Is(err, xnode.ErrInvalid) || client.calls != 3 {
		t.Fatalf("cursor loop calls=%d,%v", client.calls, err)
	}
}

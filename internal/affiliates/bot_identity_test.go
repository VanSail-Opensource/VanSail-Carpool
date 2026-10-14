package affiliates

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

type botSourceStub struct {
	user  telegram.User
	err   error
	calls int
}

func (s *botSourceStub) GetMe(context.Context) (telegram.User, error) {
	s.calls++
	return s.user, s.err
}

func TestIdentityCacheTTLAndStaleFallback(t *testing.T) {
	source := &botSourceStub{user: telegram.User{ID: 1, IsBot: true, Username: "tx_bot"}}
	cache := NewIdentityCache(source, 24*time.Hour)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if err := cache.Refresh(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if err := cache.Refresh(context.Background(), now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 {
		t.Fatalf("GetMe calls = %d, want 1", source.calls)
	}
	source.err = errors.New("temporary")
	if err := cache.Refresh(context.Background(), now.Add(25*time.Hour)); err == nil {
		t.Fatal("Refresh() returned nil")
	}
	identity := cache.Snapshot()
	if identity.Username != "tx_bot" || identity.Status != "stale" {
		t.Fatalf("identity = %#v", identity)
	}
}

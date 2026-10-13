package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestBrowserAccessLinkAtomicConsumptionAndExpiry(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 80303)
	now := time.Now().UTC()
	link := sha256.Sum256([]byte("link"))
	if err := store.CreateBrowserAccessLink(ctx, link[:], user.ID, "/admin/users", now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	outcomes := make(chan error, 2)
	start := make(chan struct{})
	for _, raw := range []string{"session-one", "session-two"} {
		go func(value string) {
			<-start
			hash := sha256.Sum256([]byte(value))
			id, destination, err := store.ExchangeBrowserAccessLink(ctx, link[:], hash[:], now, now.Add(30*24*time.Hour))
			if err == nil && (id != user.ID || destination != "/admin/users") {
				outcomes <- errors.New("wrong link owner/destination")
				return
			}
			outcomes <- err
		}(raw)
	}
	close(start)
	success, replay := 0, 0
	for range 2 {
		err := <-outcomes
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrNotFound):
			replay++
		default:
			t.Fatal(err)
		}
	}
	if success != 1 || replay != 1 {
		t.Fatalf("atomic exchange success=%d,replay=%d", success, replay)
	}
	expired := sha256.Sum256([]byte("expired"))
	session := sha256.Sum256([]byte("expired-session"))
	if err := store.CreateBrowserAccessLink(ctx, expired[:], user.ID, "/admin/settings", now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ExchangeBrowserAccessLink(ctx, expired[:], session[:], now, now.Add(30*24*time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired exchange=%v", err)
	}
	if _, _, err := store.BrowserSession(ctx, session[:], now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired link created session=%v", err)
	}
}

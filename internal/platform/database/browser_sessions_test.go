package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestBrowserSessionDeviceAndScopeIsolation(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 80301)
	now := time.Now().UTC()
	expires := now.Add(30 * 24 * time.Hour)
	first := sha256.Sum256([]byte("browser-one"))
	second := sha256.Sum256([]byte("browser-two"))
	mini := sha256.Sum256([]byte("mini-one"))
	replacement := sha256.Sum256([]byte("mini-two"))
	for _, hash := range [][32]byte{first, second} {
		if err := store.CreateBrowserSession(ctx, hash[:], user.ID, now, expires); err != nil {
			t.Fatal(err)
		}
	}
	for _, hash := range [][32]byte{mini, replacement} {
		if err := store.ReplaceSession(ctx, hash[:], user.ID, now.Add(7*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.UserBySession(ctx, first[:], now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("browser token accepted by member scope: %v", err)
	}
	if _, _, err := store.BrowserSession(ctx, replacement[:], now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("member token accepted by browser scope: %v", err)
	}
	if _, err := store.UserBySession(ctx, mini[:], now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("previous Mini App device survived replacement: %v", err)
	}
	for _, hash := range [][32]byte{first, second} {
		if _, _, err := store.BrowserSession(ctx, hash[:], now); err != nil {
			t.Fatal(err)
		}
	}
	if renewed, err := store.RenewBrowserSession(ctx, first[:], now.Add(time.Hour), expires.Add(time.Hour)); err != nil || renewed {
		t.Fatalf("early renewal = %v, %v", renewed, err)
	}
	later := now.Add(2 * 24 * time.Hour)
	if renewed, err := store.RenewBrowserSession(ctx, first[:], later, later.Add(30*24*time.Hour)); err != nil || !renewed {
		t.Fatalf("daily renewal = %v, %v", renewed, err)
	}
	if _, expiry, err := store.BrowserSession(ctx, first[:], later); err != nil || !expiry.Equal(later.Add(30*24*time.Hour)) {
		t.Fatalf("stable token expiry = %v, %v", expiry, err)
	}
	if err := store.DeleteBrowserSession(ctx, first[:]); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.BrowserSession(ctx, second[:], later); err != nil {
		t.Fatalf("other browser device revoked: %v", err)
	}
	if _, err := store.UserBySession(ctx, replacement[:], later); err != nil {
		t.Fatalf("Mini App device revoked: %v", err)
	}
}

func TestBrowserSessionMaintenanceKeepsConcurrentDevices(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 80302)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for index, raw := range []string{"old-device", "new-device"} {
		hash := sha256.Sum256([]byte(raw))
		if err := store.CreateBrowserSession(ctx, hash[:], user.ID, now.Add(time.Duration(index-2)*time.Hour), now.Add(30*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	mini := sha256.Sum256([]byte("member-device"))
	if err := store.ReplaceSession(ctx, mini[:], user.ID, now.Add(7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id=?`, user.ID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("retained sessions=%d,err=%v", count, err)
	}
}

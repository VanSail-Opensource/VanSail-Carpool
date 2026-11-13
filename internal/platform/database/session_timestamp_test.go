package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionAuthenticationUsesExactExpiryBeforeActivity(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"miniapp", "browser"} {
		for _, live := range []bool{false, true} {
			t.Run(scope+map[bool]string{false: "/expired", true: "/live"}[live], func(t *testing.T) {
				t.Parallel()
				ctx, store := context.Background(), newTestStore(t)
				user := createTestUser(t, store, 497721)
				boundary := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
				expires, at := boundary, boundary.Add(900*time.Nanosecond)
				if live {
					expires, at = at, expires
				}
				hash := make([]byte, 32)
				var err error
				if scope == "browser" {
					err = store.CreateBrowserSession(ctx, hash, user.ID, boundary.Add(-time.Hour), expires)
				} else {
					err = store.ReplaceSession(ctx, hash, user.ID, expires)
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.DB().ExecContext(ctx, `UPDATE users SET bot_blocked=1 WHERE id=$1`, user.ID); err != nil {
					t.Fatal(err)
				}
				got, expiry, err := store.authenticatedSession(ctx, hash, scope, at)
				if live {
					if err != nil || got.ID != user.ID || !expiry.Equal(expires) {
						t.Fatal("live fractional expiry was rejected", got, expiry, err)
					}
				} else if !errors.Is(err, ErrNotFound) {
					t.Fatal("expired whole-second session was authenticated", err)
				}
				var blocked bool
				if err := store.DB().QueryRowContext(ctx, `SELECT bot_blocked=1 FROM users WHERE id=$1`, user.ID).Scan(&blocked); err != nil || blocked == live {
					t.Fatal("expired session changed authenticated activity", blocked, err)
				}
			})
		}
	}
}

func TestBrowserSessionRenewalCannotReviveExpiredFractionalBoundary(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 497722)
	boundary := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	hash := make([]byte, 32)
	if err := store.CreateBrowserSession(ctx, hash, user.ID, boundary.Add(-25*time.Hour), boundary); err != nil {
		t.Fatal(err)
	}
	at := boundary.Add(900 * time.Nanosecond)
	if renewed, err := store.RenewBrowserSession(ctx, hash, at, at.Add(30*24*time.Hour)); err != nil || renewed {
		t.Fatal("expired bearer was renewed", renewed, err)
	}
	if _, _, err := store.BrowserSession(ctx, hash, at); !errors.Is(err, ErrNotFound) {
		t.Fatal("renewal revived expired bearer", err)
	}
}

func TestBrowserHandoffUsesExactExpiry(t *testing.T) {
	t.Parallel()
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired", true: "live"}[live], func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 497723)
			boundary := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
			expires, at := boundary, boundary.Add(900*time.Nanosecond)
			if live {
				expires, at = at, expires
			}
			link, session := make([]byte, 32), make([]byte, 32)
			if err := store.CreateBrowserAccessLink(ctx, link, user.ID, "/", expires); err != nil {
				t.Fatal(err)
			}
			owner, _, err := store.ExchangeBrowserAccessLink(ctx, link, session, at, at.Add(time.Hour), allowBrowserTestUser)
			if live {
				if err != nil || owner.ID != user.ID {
					t.Fatal("live fractional handoff rejected", owner, err)
				}
			} else if !errors.Is(err, ErrNotFound) {
				t.Fatal("expired whole-second handoff accepted", err)
			}
			var sessions int
			if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE user_id=$1`, user.ID).Scan(&sessions); err != nil || sessions != boolInt(live) {
				t.Fatal("handoff created an incorrect session count", sessions, err)
			}
		})
	}
}

package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func TestAPITokenLimitUsesExactExpiry(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		now     time.Time
		expires time.Time
		limited bool
	}{
		{"live_fraction_after_whole_second", base, base.Add(time.Nanosecond), true},
		{"expired_whole_before_fraction", base.Add(time.Nanosecond), base, false},
		{"expiry_is_exclusive", base, base, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newTestStore(t)
			ctx := context.Background()
			actor := createTestUser(t, store, 81704)
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
				t.Fatal(err)
			}
			makeToken := func(at time.Time, expires *time.Time) apitoken.Token {
				id, err := ids.New()
				if err != nil {
					t.Fatal(err)
				}
				hash := sha256.Sum256([]byte(id))
				return apitoken.Token{ID: id, UserID: actor.ID, Name: "expiry boundary", Prefix: "vsc_boundary", Hash: hash[:], Scopes: []string{"admin:read"}, CreatedAt: at, ExpiresAt: expires}
			}
			for range 50 {
				if err := store.CreateAdminAPIToken(ctx, makeToken(base.Add(-time.Hour), &tc.expires)); err != nil {
					t.Fatal("seed active token", err)
				}
			}
			candidate := makeToken(tc.now, nil)
			err := store.CreateAdminAPIToken(ctx, candidate)
			if tc.limited && !errors.Is(err, apitoken.ErrLimit) || !tc.limited && err != nil {
				t.Fatal("wrong exact-expiry admission", tc.limited, err)
			}
			want := 1
			if tc.limited {
				want = 0
			}
			var count int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_api_tokens WHERE id=$1`, candidate.ID).Scan(&count); err != nil || count != want {
				t.Fatal("wrong credential mutation", count, want, err)
			}
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='api_token.created' AND target_type='api_token' AND target_id=$2`, actor.ID, candidate.ID).Scan(&count); err != nil || count != want {
				t.Fatal("wrong admission audit", count, want, err)
			}
		})
	}
}

func TestAPITokenUsageThrottleKeepsExactBoundary(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestAdmin(t, store, 81705)
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	for _, scenario := range []struct {
		id          string
		now, used   time.Time
		wantTouched bool
	}{
		{"fractional-not-due", base.Add(5 * time.Minute), base.Add(time.Nanosecond), false},
		{"whole-already-due", base.Add(5*time.Minute + time.Nanosecond), base, true},
	} {
		hash := sha256.Sum256([]byte(scenario.id))
		item := apitoken.Token{ID: scenario.id, UserID: actor.ID, Name: "touch boundary", Prefix: "vsc_boundary",
			Hash: hash[:], Scopes: []string{"admin:read"}, CreatedAt: base.Add(-time.Hour)}
		if err := store.CreateAdminAPIToken(ctx, item); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().ExecContext(ctx, `UPDATE admin_api_tokens SET last_used_at=$1 WHERE id=$2`, stamp(scenario.used), item.ID); err != nil {
			t.Fatal(err)
		}
		if err := store.TouchAdminAPIToken(ctx, item.ID, scenario.now); err != nil {
			t.Fatal(err)
		}
		current, err := store.AdminAPITokenByHash(ctx, hash[:])
		want := scenario.used
		if scenario.wantTouched {
			want = scenario.now
		}
		if err != nil || current.LastUsedAt == nil || !current.LastUsedAt.Equal(want) {
			t.Fatalf("%s usage = %+v, %v; want %s", scenario.id, current.LastUsedAt, err, want)
		}
	}
}

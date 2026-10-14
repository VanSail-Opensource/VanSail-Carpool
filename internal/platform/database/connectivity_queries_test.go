package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

func TestConnectivityHistoryStablePaginationLimitsAndFilterBinding(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	for index := 1; index <= 202; index++ {
		seedConnectivityAttempt(t, store, connectivityTestAttempt(index, connectivityTestHost, now.Add(-time.Minute)))
	}
	first, err := store.ConnectivityHistory(ctx, "", "", 0, now)
	if err != nil || len(first.Items) != 50 || first.NextCursor == nil {
		t.Fatalf("default page=%+v,%v", first, err)
	}
	if first.Items[0].ID != connectivityTestAttempt(202, "", now).ID || first.Items[49].ID != connectivityTestAttempt(153, "", now).ID {
		t.Fatalf("equal-time order=%s through %s", first.Items[0].ID, first.Items[49].ID)
	}
	// A newer insert must neither repeat nor shift already traversed records.
	seedConnectivityAttempt(t, store, connectivityTestAttempt(203, connectivityTestHost, now))
	second, err := store.ConnectivityHistory(ctx, "", *first.NextCursor, 200, now)
	if err != nil || len(second.Items) != 152 || second.NextCursor != nil || second.Items[0].ID != connectivityTestAttempt(152, "", now).ID {
		t.Fatalf("continued page=%+v,%v", second, err)
	}
	large, err := store.ConnectivityHistory(ctx, "", "", 500, now)
	if err != nil || len(large.Items) != 200 || large.NextCursor == nil {
		t.Fatalf("maximum page=%+v,%v", large, err)
	}
	filtered, err := store.ConnectivityHistory(ctx, connectivityTestHost, "", 1, now)
	if err != nil || filtered.NextCursor == nil {
		t.Fatalf("filtered page=%+v,%v", filtered, err)
	}
	if _, err := store.ConnectivityHistory(ctx, connectivityOtherHost, *filtered.NextCursor, 1, now); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cross-filter cursor=%v", err)
	}
	if _, err := store.ConnectivityHistory(ctx, "", "bad cursor", 1, now); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("invalid cursor=%v", err)
	}
	if _, err := store.ConnectivityHistory(ctx, "not-a-host-uuid", "", 1, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("invalid host filter=%v", err)
	}
}

func TestConnectivityHistoryOrdersWholeSecondsAndFractionsAcrossCursors(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	for index, offset := range []time.Duration{0, time.Nanosecond, 10 * time.Nanosecond, time.Millisecond, time.Second} {
		seedConnectivityAttempt(t, store, connectivityTestAttempt(index+1, connectivityTestHost, now.Add(-time.Minute+offset)))
	}
	cursor := ""
	for expected := 5; expected >= 1; expected-- {
		page, err := store.ConnectivityHistory(ctx, connectivityTestHost, cursor, 1, now)
		if err != nil || len(page.Items) != 1 || page.Items[0].ID != connectivityTestAttempt(expected, "", now).ID {
			t.Fatalf("fractional page %d=%+v,%v", expected, page, err)
		}
		if expected == 1 {
			if page.NextCursor != nil {
				t.Fatal("last page still has cursor")
			}
			continue
		}
		if page.NextCursor == nil {
			t.Fatal("missing intermediate cursor")
		}
		cursor = *page.NextCursor
	}
}

func TestLatestConnectivityAttemptsSelectsConfigurationAndStableHostLatest(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	a := connectivityTestAttempt(1, connectivityTestHost, now.Add(-time.Minute))
	b := connectivityTestAttempt(2, connectivityTestHost, a.StartedAt)
	c := connectivityTestAttempt(3, connectivityOtherHost, now.Add(-2*time.Minute))
	d := connectivityTestAttempt(4, connectivityTestHost, now)
	d.ConfigHash = strings.Repeat("b", 64)
	for _, attempt := range []connectivity.Attempt{a, b, c, d, connectivityTestAttempt(5, "", now)} {
		seedConnectivityAttempt(t, store, attempt)
	}
	latest, err := store.LatestConnectivityAttempts(ctx, a.ConfigHash, now)
	if err != nil || len(latest) != 2 || latest[0].ID != b.ID || latest[1].ID != c.ID {
		t.Fatalf("current configuration latest=%+v,%v", latest, err)
	}
	other, err := store.LatestConnectivityAttempts(ctx, d.ConfigHash, now)
	if err != nil || len(other) != 1 || other[0].ID != d.ID {
		t.Fatalf("other configuration latest=%+v,%v", other, err)
	}
	missing, err := store.LatestConnectivityAttempts(ctx, strings.Repeat("c", 64), now)
	if err != nil || missing == nil || len(missing) != 0 {
		t.Fatalf("missing configuration latest=%+v,%v", missing, err)
	}
}

package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAddTXBBoundsReadUpdateAndAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	actor := createTestAdmin(t, store, 50_003)
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)

	defaults, err := store.AddTXBBounds(ctx)
	if err != nil || defaults.MinimumTXBMinor != DefaultAddTXBMinimumMinor || defaults.MaximumTXBMinor != DefaultAddTXBMaximumMinor {
		t.Fatalf("AddTXBBounds(defaults) = (%+v, %v)", defaults, err)
	}
	invalid := []struct {
		name             string
		minimum, maximum int64
	}{
		{name: "zero minimum", minimum: 0, maximum: 500},
		{name: "inverted range", minimum: 501, maximum: 500},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.UpdateAddTXBBounds(ctx, test.minimum, test.maximum, actor.ID, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("UpdateAddTXBBounds() error = %v, want ErrConflict", err)
			}
		})
	}

	updated, err := store.UpdateAddTXBBounds(ctx, 250, 25_000, actor.ID, now)
	if err != nil || updated.MinimumTXBMinor != 250 || updated.MaximumTXBMinor != 25_000 || !updated.UpdatedAt.Equal(now) {
		t.Fatalf("UpdateAddTXBBounds() = (%+v, %v)", updated, err)
	}
	loaded, err := store.AddTXBBounds(ctx)
	if err != nil || loaded.MinimumTXBMinor != 250 || loaded.MaximumTXBMinor != 25_000 {
		t.Fatalf("AddTXBBounds(updated) = (%+v, %v)", loaded, err)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1
		AND action='billing.amount_bounds.update' AND target_type='txb_limits' AND target_id='1'`, actor.ID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("bounds audit count = %d, want 1", audits)
	}
}

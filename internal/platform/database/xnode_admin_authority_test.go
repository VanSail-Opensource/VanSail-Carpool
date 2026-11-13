package database

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNativeAdminWritesRecheckCurrentAuthority(t *testing.T) {
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0}, {"purging", "admin", "purging", 0}, {"blocked", "admin", "active", 1},
		{"missing", "admin", "active", 0}, {"empty", "admin", "active", 0},
	} {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			f := newXNodeAdminFixture(t)
			if _, err := f.store.DB().ExecContext(context.Background(), `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, state.role, state.status, state.blocked, f.actor); err != nil {
				t.Fatal(err)
			}
			actor := f.actor
			if state.name == "missing" {
				actor = "missing-administrator"
			}
			if state.name == "empty" {
				actor = ""
			}
			before := xnodeAdminSnapshot(t, f.store)
			for _, attempt := range f.attempts() {
				if err := attempt.write(actor); !errors.Is(err, xnode.ErrConflict) {
					t.Fatalf("%s accepted stale/missing administrator: %v", attempt.name, err)
				}
			}
			if after := xnodeAdminSnapshot(t, f.store); after != before {
				t.Fatal("rejected administrator changed native state or audit evidence")
			}
		})
	}
}

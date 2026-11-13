package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestActivityMutationsRequireCurrentEligibleAdministrator(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	actor := activityAdminActor(t, store)
	game, err := store.SaveActivityGame(ctx, activity.GameInput{Name: "Protected", Icon: "coin", Enabled: true, WinChanceBPS: 5000,
		MinimumStakeMinor: 100, MaximumStakeMinor: 500, ReturnMultiplierBPS: 20000}, now)
	if err != nil {
		t.Fatal(err)
	}
	draw, err := store.SaveLuckyDraw(ctx, auditRaffleInput("protected", "protected_draw"), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0}, {"purging", "admin", "purging", 0}, {"blocked", "admin", "active", 1},
	} {
		t.Run(state.name, func(t *testing.T) {
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, state.role, state.status, state.blocked, actor); err != nil {
				t.Fatal(err)
			}
			if _, err := store.SaveAdminActivityGame(ctx, actor, game.GameInput, now); !errors.Is(err, ErrConflict) {
				t.Fatal("game actor fence", err)
			}
			if _, err := store.SaveAdminLuckyDraw(ctx, actor, draw.LuckyDrawInput, now); !errors.Is(err, ErrConflict) {
				t.Fatal("draw actor fence", err)
			}
			if err := store.PublishAdminRaffle(ctx, actor, draw.ID, -100123, now); !errors.Is(err, ErrConflict) {
				t.Fatal("publish actor fence", err)
			}
			if err := store.DeleteActivityGame(ctx, actor, game.ID, now); !errors.Is(err, ErrConflict) {
				t.Fatal("game deletion actor fence", err)
			}
			if err := store.DeleteLuckyDraw(ctx, actor, draw.ID, now); !errors.Is(err, ErrConflict) {
				t.Fatal("draw deletion actor fence", err)
			}
			if _, err := store.SaveActivitySettings(ctx, actor, activity.CheckInConfig{Timezone: "UTC"}, now); !errors.Is(err, ErrConflict) {
				t.Fatal("settings actor fence", err)
			}
		})
	}
	stored, err := store.LuckyDrawByID(ctx, draw.ID)
	if err != nil || stored.Revision != 1 || stored.Status != "draft" {
		t.Fatal(stored, err)
	}
	games, err := store.ListActivityGames(ctx, true)
	if err != nil || len(games) != 1 || games[0].ID != game.ID {
		t.Fatal(games, err)
	}
	settings, err := store.ListSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, setting := range settings {
		if setting.Key == ActivityTimezoneSetting {
			t.Fatal("rejected actor published settings")
		}
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1`, actor).Scan(&audits); err != nil || audits != 0 {
		t.Fatal(audits, err)
	}
}

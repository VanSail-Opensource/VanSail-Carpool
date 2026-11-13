package database

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
)

func TestAPITokenWritesRecheckStaleAdministratorSnapshot(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 89125)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	actor.Role = "admin"
	service := apitoken.New(store, []int64{actor.TelegramID})
	input := apitoken.CreateInput{Name: "retained", Scopes: []string{"admin:read"}}
	issued, err := service.Create(ctx, actor, input)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []struct {
		role, status string
		blocked      int
	}{{"user", "active", 0}, {"admin", "purging", 0}, {"admin", "active", 1}} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, state.role, state.status, state.blocked, actor.ID); err != nil {
			t.Fatal(err)
		}
		// actor intentionally retains the previously authorized request snapshot.
		if _, err := service.Create(ctx, actor, input); !errors.Is(err, apitoken.ErrUnauthorized) {
			t.Fatal("stale actor minted a new credential", state, err)
		}
		if err := service.Revoke(ctx, actor, issued.Item.ID); !errors.Is(err, apitoken.ErrUnauthorized) {
			t.Fatal("stale actor revoked a credential", state, err)
		}
		if _, err := service.Authenticate(ctx, issued.Token); !errors.Is(err, apitoken.ErrUnauthorized) {
			t.Fatal("current ineligible owner authenticated", state, err)
		}
	}
	var tokens, audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM admin_api_tokens WHERE user_id=$1),(SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND target_type='api_token')`, actor.ID).Scan(&tokens, &audits); err != nil || tokens != 1 || audits != 1 {
		t.Fatal("rejected actor retained a mutation or audit", tokens, audits, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin',account_status='active',bot_blocked=0 WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := service.Revoke(ctx, actor, issued.Item.ID); err != nil {
			t.Fatal("restored owner lost idempotent revocation", err)
		}
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM admin_api_tokens WHERE user_id=$1),(SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND target_type='api_token' AND action='api_token.revoked' AND target_id=$2)`, actor.ID, issued.Item.ID).Scan(&tokens, &audits); err != nil || tokens != 0 || audits != 1 {
		t.Fatal("revocation did not retain one audit after deleting credential", tokens, audits, err)
	}
}

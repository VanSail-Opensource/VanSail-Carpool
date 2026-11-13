package database

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestManualGrantFreshAuthorityAndOriginalCreationReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, actor, owner, combo, now := manualGrantFixture(t)
	input := manualGrantInput(actor.ID, owner.ID, combo.ID, now)
	accepted, err := store.CreateAdminEntitlement(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	var audit string
	if err = store.DB().QueryRowContext(ctx, `SELECT detail FROM audit_events WHERE action='entitlement.create' AND target_id=$1`, accepted.ID).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	// Current display fields may legitimately change. A retry of the creation
	// command must still describe the accepted zero-charge grant.
	if _, err = store.DB().ExecContext(ctx, `UPDATE combos SET name='later catalog name',active=0 WHERE id=$1`, combo.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1,status='cancelled',entitlement_traffic_limit_bytes=999,updated_at=$2 WHERE id=$3`, stamp(input.ValidUntil.Add(time.Hour)), stamp(now.Add(time.Minute)), accepted.ID); err != nil {
		t.Fatal(err)
	}
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0}, {"purging", "admin", "purging", 0}, {"blocked", "admin", "active", 1},
	} {
		if _, err = store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, state.role, state.status, state.blocked, actor.ID); err != nil {
			t.Fatal(err)
		}
		fresh := input
		fresh.IdempotencyKey = state.name
		if _, err = store.CreateAdminEntitlement(ctx, fresh, now); !errors.Is(err, ErrConflict) {
			t.Fatal("fresh grant ignored current administrator eligibility", state.name, err)
		}
		replay, err := store.CreateAdminEntitlement(ctx, input, now.Add(time.Minute))
		if err != nil || !reflect.DeepEqual(replay, accepted) {
			t.Fatal("manual grant replay returned later mutable state", replay, accepted, err)
		}
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE users SET role='admin',account_status='active',bot_blocked=0 WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, owner.ID); err != nil {
		t.Fatal(err)
	}
	fresh := input
	fresh.IdempotencyKey = "purging-owner"
	if _, err = store.CreateAdminEntitlement(ctx, fresh, now); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("fresh grant ignored owner lifecycle", err)
	}
	replay, found, err := store.AdminEntitlementGrantByKey(ctx, actor.ID, owner.ID, input.IdempotencyKey)
	if err != nil || !found || !reflect.DeepEqual(replay.Purchase, accepted) || replay.RequestFingerprint != input.RequestFingerprint {
		t.Fatal("creation evidence lookup changed", replay, found, err)
	}
	var after string
	var purchases, audits, jobs int
	if err = store.DB().QueryRowContext(ctx, `SELECT detail FROM audit_events WHERE action='entitlement.create' AND target_id=$1`, accepted.ID).Scan(&after); err != nil || after != audit {
		t.Fatal("creation audit changed", after, err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM purchases WHERE user_id=$1),
		(SELECT COUNT(*) FROM audit_events WHERE action='entitlement.create' AND target_id=$2),
		(SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user' AND payload::jsonb->>'userId'=$1)`, owner.ID, accepted.ID).Scan(&purchases, &audits, &jobs); err != nil || purchases != 1 || audits != 1 || jobs != 1 {
		t.Fatal("rejected/replayed grant created work", purchases, audits, jobs, err)
	}
}

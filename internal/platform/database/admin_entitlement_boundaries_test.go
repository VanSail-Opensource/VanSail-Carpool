package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManualGrantRetainsExactLongDurationAndReplay(t *testing.T) {
	t.Parallel()
	store, actor, owner, combo, now := manualGrantFixture(t)
	input := manualGrantInput(actor.ID, owner.ID, combo.ID, now)
	input.ValidFrom = now.Add(500 * time.Millisecond)
	input.ValidUntil = time.Date(2500, 1, 1, 0, 0, 0, 400000000, time.UTC)
	want := input.ValidUntil.Unix() - input.ValidFrom.Unix() - 1
	for range 2 {
		purchase, err := store.CreateAdminEntitlement(context.Background(), input, now)
		if err != nil || purchase.PurchasedTermSeconds == nil || *purchase.PurchasedTermSeconds != want {
			t.Fatalf("accepted long term evidence = %+v, %v; want %d seconds", purchase, err, want)
		}
	}
}

func TestManualGrantRejectsOneNanosecondOverlap(t *testing.T) {
	t.Parallel()
	store, actor, owner, combo, now := manualGrantFixture(t)
	input := manualGrantInput(actor.ID, owner.ID, combo.ID, now)
	input.ValidUntil = now.Add(time.Hour + time.Nanosecond)
	if _, err := store.CreateAdminEntitlement(context.Background(), input, now); err != nil {
		t.Fatal(err)
	}
	input.IdempotencyKey = "nanosecond-overlap"
	input.ValidFrom, input.ValidUntil = now.Add(time.Hour), now.Add(2*time.Hour)
	if _, err := store.CreateAdminEntitlement(context.Background(), input, now); !errors.Is(err, ErrEntitlementOverlap) {
		t.Fatalf("one-nanosecond overlap = %v", err)
	}
	input.ValidFrom = input.ValidFrom.Add(time.Nanosecond)
	if _, err := store.CreateAdminEntitlement(context.Background(), input, now); err != nil {
		t.Fatalf("exactly adjacent grant rejected: %v", err)
	}
}

func TestAdminBulkAndReplacementUseExactPaidWindow(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2030, 10, 8, 0, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "Exact admin window", 100, 30)
	actor := createTestAdmin(t, store, 49131)
	user, purchase := createAdminWorkflowPurchase(t, store, 49132, combo, now)
	for _, scenario := range []struct {
		name            string
		from, until, at time.Time
		want            int
	}{
		{"fractional future expiry", now.Add(-time.Hour), now.Add(time.Nanosecond), now, 1},
		{"fractional future start", now.Add(time.Nanosecond), now.Add(time.Hour), now, 0},
		{"whole expired before fractional read", now.Add(-time.Hour), now, now.Add(500 * time.Millisecond), 0},
		{"whole start before fractional read", now, now.Add(time.Hour), now.Add(500 * time.Millisecond), 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(scenario.from), stamp(scenario.until), purchase.ID); err != nil {
				t.Fatal(err)
			}
			preview, err := store.PreviewAdminBulkExtension(ctx, AdminBulkExtensionFilter{ComboIDs: []string{combo.ID}}, scenario.at)
			if err != nil || preview.MatchedUsers != scenario.want {
				t.Fatalf("exact eligible targets = %+v, %v; want %d", preview, err, scenario.want)
			}
			clause, args := adminUserFacetClause(AdminUserSearchFilter{State: "active", Match: "and"}, scenario.at, 2)
			var visible bool
			if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND (`+clause+`))`,
				append([]any{user.ID}, args...)...).Scan(&visible); err != nil || visible != (scenario.want == 1) {
				t.Fatalf("exact active member facet = %v, %v", visible, err)
			}
		})
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(now.Add(-time.Hour)), stamp(now.Add(time.Nanosecond)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReplaceAdminCombo(ctx, AdminComboReplacementInput{ActorUserID: actor.ID, UserID: user.ID, ComboID: combo.ID,
		IdempotencyKey: "fractional-replace", RequestFingerprint: "fractional-replace-fingerprint", Reason: "replace current term"}, now); err != nil {
		t.Fatalf("replacement skipped still-paid fractional term: %v", err)
	}
	var addons string
	if err := store.DB().QueryRowContext(ctx, `SELECT entitlement_addon_squad_uuids FROM purchases WHERE id=$1`, purchase.ID).Scan(&addons); err != nil || addons != "[]" {
		t.Fatalf("omitted replacement add-ons = %q, %v; want an explicit empty selection", addons, err)
	}
	if _, err := store.CreateAdminBulkExtension(ctx, AdminBulkExtensionInput{ActorUserID: actor.ID, IdempotencyKey: "fractional-extend",
		RequestFingerprint: "fractional-extend-fingerprint", Reason: "extend current term", Filter: AdminBulkExtensionFilter{ComboIDs: []string{combo.ID}}, DurationMinutes: 1}, now); err != nil {
		t.Fatalf("extension skipped still-paid fractional term: %v", err)
	}
	current, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || !current.ValidUntil.Equal(now.Add(time.Minute+time.Nanosecond)) {
		t.Fatalf("exact extension = %+v, %v", current, err)
	}
}

func TestAdminEntitlementDatesRejectUnrepresentableUTCYears(t *testing.T) {
	start := time.Date(9999, 12, 31, 23, 59, 30, 0, time.UTC)
	if _, err := addSubscriptionMinutes(start, 1); !errors.Is(err, ErrConflict) {
		t.Fatal("extension wrote an unrepresentable stamp", err)
	}
	if _, err := adminEntitlementTermSeconds(start, start.Add(time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatal("manual grant accepted an unrepresentable term", err)
	}
}

func TestManualGrantReplayPreservesHistoricalDurationEvidence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, actor, owner, combo, now := manualGrantFixture(t)
	input := manualGrantInput(actor.ID, owner.ID, combo.ID, now)
	accepted, err := store.CreateAdminEntitlement(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []struct {
		key  string
		term any
	}{
		{"historical-missing", nil}, {"historical-retained", int64(123)},
	} {
		if _, err := store.DB().ExecContext(ctx, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,
			valid_from,valid_until,status,idempotency_key,request_fingerprint,created_at,updated_at,purchased_term_seconds)
			SELECT $1,user_id,combo_id,0,valid_from,valid_until,status,$2,request_fingerprint,created_at,updated_at,$3
			FROM purchases WHERE id=$4`, legacy.key, adminGrantKeyPrefix+actor.ID+":"+legacy.key, legacy.term, accepted.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
			SELECT $1,actor_user_id,action,target_type,$1,detail,created_at FROM audit_events
			WHERE action='entitlement.create' AND target_id=$2`, legacy.key, accepted.ID); err != nil {
			t.Fatal(err)
		}
		replay, found, err := store.AdminEntitlementGrantByKey(ctx, actor.ID, owner.ID, legacy.key)
		if err != nil || !found {
			t.Fatal("historical grant replay failed", found, err)
		}
		term := replay.Purchase.PurchasedTermSeconds
		if legacy.term == nil && term != nil || legacy.term != nil && (term == nil || *term != legacy.term.(int64)) {
			t.Fatalf("historical duration evidence changed: %+v, want %v", term, legacy.term)
		}
	}
}

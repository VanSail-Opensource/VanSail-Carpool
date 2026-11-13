package database

import (
	"context"
	"errors"
	"testing"
)

func TestPaymentProfileMutationsRequireCurrentEligibleActor(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 89124)
	input := PaymentProfileInput{ID: "actor-account", Provider: "ezpay", ProviderName: "Before", Endpoint: "https://account.example", CredentialCiphertext: "ciphertext", Enabled: true}
	initial, err := store.SavePaymentProfile(ctx, input)
	if err != nil {
		t.Fatal("system bootstrap failed", err)
	}
	input.ActorID, input.ProviderName = actor.ID, "After"
	for _, state := range []struct {
		role, status string
		blocked      int
	}{{"user", "active", 0}, {"admin", "purging", 0}, {"admin", "active", 1}} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, state.role, state.status, state.blocked, actor.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.SavePaymentProfile(ctx, input); !errors.Is(err, ErrConflict) {
			t.Fatal("ineligible actor changed payment account", state, err)
		}
		if err := store.DeletePaymentProfile(ctx, input.ID, actor.ID); !errors.Is(err, ErrConflict) {
			t.Fatal("ineligible actor deleted payment account", state, err)
		}
	}
	stored, err := store.PaymentProfileRecordByID(ctx, input.ID, "")
	if err != nil || stored.ProviderName != "Before" || stored.CredentialCiphertext != "ciphertext" || !stored.UpdatedAt.Equal(initial.UpdatedAt) {
		t.Fatal("rejected actor changed retained profile", stored.PaymentProfile, err)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND target_type='payment_profile' AND target_id=$2`, actor.ID, input.ID).Scan(&audits); err != nil || audits != 0 {
		t.Fatal("rejected actor left a payment account audit", audits, err)
	}
	// Automatic discovery remains a system operation, independent of which
	// administrator originally configured the profile.
	if _, err := store.SetPaymentProfileEnabled(ctx, input.ID, false); err != nil {
		t.Fatal("system readiness could not disable the account", err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin',account_status='active',bot_blocked=0 WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SavePaymentProfile(ctx, input); err != nil {
		t.Fatal("eligible administrator could not save account", err)
	}
	if err := store.DeletePaymentProfile(ctx, input.ID, actor.ID); err != nil {
		t.Fatal("eligible administrator could not remove unreferenced account", err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action IN ('payment_profile.update','payment_profile.delete') AND target_type='payment_profile' AND target_id=$2`, actor.ID, input.ID).Scan(&audits); err != nil || audits != 2 {
		t.Fatal("eligible profile writes lost actor audit", audits, err)
	}
}

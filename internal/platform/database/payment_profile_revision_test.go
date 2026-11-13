package database

import (
	"context"
	"testing"
)

func TestPaymentDiscoveryCannotDisableANewerProfileRevision(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	input := PaymentProfileInput{ID: "discovered-profile", Provider: "ezpay", ProviderName: "Original",
		Endpoint: "https://account.example", CredentialCiphertext: "ciphertext", Enabled: true}
	observed, err := store.SavePaymentProfile(ctx, input)
	if err != nil || observed.UpdatedAt.IsZero() {
		t.Fatal("missing initial profile revision", err)
	}
	input.ProviderName, input.CredentialCiphertext = "Edited", ""
	edited, err := store.SavePaymentProfile(ctx, input)
	if err != nil || !edited.UpdatedAt.After(observed.UpdatedAt) {
		t.Fatal("save did not advance profile revision", err)
	}
	current, disabled, err := store.DisablePaymentProfileIfUnchanged(ctx, input.ID, observed.UpdatedAt)
	if err != nil || disabled || !current.Enabled || current.ProviderName != "Edited" || !current.UpdatedAt.Equal(edited.UpdatedAt) {
		t.Fatalf("stale discovery overwrote edit: disabled=%v current=%+v err=%v", disabled, current, err)
	}
	current, disabled, err = store.DisablePaymentProfileIfUnchanged(ctx, input.ID, edited.UpdatedAt)
	if err != nil || !disabled || current.Enabled || !current.UpdatedAt.After(edited.UpdatedAt) {
		t.Fatalf("current discovery was not applied: disabled=%v current=%+v err=%v", disabled, current, err)
	}
	disabledRevision := current.UpdatedAt
	current, disabled, err = store.DisablePaymentProfileIfUnchanged(ctx, input.ID, edited.UpdatedAt)
	if err != nil || disabled || current.Enabled || !current.UpdatedAt.Equal(disabledRevision) {
		t.Fatal("replayed discovery changed an already disabled revision", err)
	}
	reenabled, err := store.SetPaymentProfileEnabled(ctx, input.ID, true)
	if err != nil || !reenabled.Enabled || !reenabled.UpdatedAt.After(disabledRevision) {
		t.Fatal("re-enable did not advance revision", err)
	}
	current, disabled, err = store.DisablePaymentProfileIfUnchanged(ctx, input.ID, edited.UpdatedAt)
	if err != nil || disabled || !current.Enabled || !current.UpdatedAt.Equal(reenabled.UpdatedAt) {
		t.Fatal("stale discovery disabled a re-enabled account", err)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM audit_events WHERE target_type='payment_profile' AND target_id=$1 AND action='payment_profile.availability'`, input.ID).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("availability audit count=%d, %v; want successful disable and re-enable only", audits, err)
	}
	profiles, err := store.ListPaymentProfiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		if profile.ID == input.ID {
			if !profile.UpdatedAt.Equal(reenabled.UpdatedAt) {
				t.Fatal("discovery list omitted current revision")
			}
			return
		}
	}
	t.Fatal("profile missing from discovery list")
}

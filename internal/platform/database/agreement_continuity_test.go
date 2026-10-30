package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/onboarding"
)

func TestAgreementPublicationPreservesAcceptedMembers(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	existing := createTestUser(t, store, 31931)
	newcomer := createTestUser(t, store, 31932)
	bundle, err := store.OnboardingBundle(ctx, onboarding.KindAgreements)
	if err != nil {
		t.Fatal(err)
	}
	accepted := time.Now().UTC().Add(-time.Hour)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete',policy_accepted_at=?,accepted_agreement_revision=? WHERE id=?`, stamp(accepted), bundle.PublishedRevision, existing.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginAgreement(ctx, newcomer.ID); err != nil {
		t.Fatal(err)
	}
	oldRevision, oldIDs, err := store.CurrentAgreementContract(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for locale, raw := range bundle.Draft {
		var items []onboarding.Agreement
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatal(err)
		}
		items[0].Body += " updated"
		encoded, err := json.Marshal(items)
		if err != nil {
			t.Fatal(err)
		}
		bundle.Draft[locale] = encoded
	}
	bundle, err = store.SaveOnboardingDraft(ctx, bundle.Kind, bundle.Draft, bundle.DraftRevision, existing.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	published, err := store.PublishOnboarding(ctx, bundle.Kind, bundle.DraftRevision, existing.ID, time.Now())
	if err != nil || published.PublishedRevision != oldRevision+1 {
		t.Fatal(published, err)
	}
	got, err := store.UserByID(ctx, existing.ID)
	if err != nil || got.OnboardingState != "complete" || got.AgreementRevision != oldRevision || got.PolicyAcceptedAt == nil || !got.PolicyAcceptedAt.Equal(accepted) {
		t.Fatal("acceptance changed", got, err)
	}
	if _, err := store.CompleteOnboardingRevision(ctx, newcomer.ID, oldRevision, oldIDs, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatal("stale initial acceptance accepted", err)
	}
	revision, ids, err := store.CurrentAgreementContract(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteOnboardingRevision(ctx, newcomer.ID, revision, ids, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestAgreementContinuityMigrationRestoresOnlyPriorAcceptance(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	accepted := createTestUser(t, store, 31933)
	newcomer := createTestUser(t, store, 31934)
	for _, id := range []string{accepted.ID, newcomer.ID} {
		if err := store.BeginAgreement(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET accepted_agreement_revision=1 WHERE id=?`, accepted.ID); err != nil {
		t.Fatal(err)
	}
	script, err := migrations.ReadFile("migrations/113_agreement_continuity.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := store.DB().ExecContext(ctx, string(script)); err != nil {
			t.Fatal(err)
		}
	}
	for id, state := range map[string]string{accepted.ID: "complete", newcomer.ID: "agreement"} {
		got, err := store.UserByID(ctx, id)
		if err != nil || got.OnboardingState != state {
			t.Fatal(got, err)
		}
	}
}

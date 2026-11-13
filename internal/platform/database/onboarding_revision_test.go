package database

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/onboarding"
)

func TestOnboardingDraftCASAndPublicationUseExactRevision(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	actor := createTestAdmin(t, s, 970003)
	before, err := s.OnboardingBundle(ctx, onboarding.KindWelcome)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for _, text := range []string{"first editor", "second editor"} {
		group.Go(func() {
			written, err := s.SaveOnboardingDraft(ctx, before.Kind, onboardingAdminTestContent(text), before.DraftRevision, actor.ID, time.Now())
			if err == nil && written.DraftRevision != before.DraftRevision+1 {
				t.Error("write returned an unexpected committed revision", written.DraftRevision)
			}
			results <- err
		})
	}
	group.Wait()
	close(results)
	won, refused := 0, 0
	for err := range results {
		switch {
		case err == nil:
			won++
		case errors.Is(err, ErrConflict):
			refused++
		default:
			t.Fatal(err)
		}
	}
	if won != 1 || refused != 1 {
		t.Fatal("stale editors did not conflict", won, refused)
	}
	if _, err = s.PublishOnboarding(ctx, before.Kind, before.DraftRevision, actor.ID, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatal("stale publication succeeded", err)
	}
	published, err := s.PublishOnboarding(ctx, before.Kind, before.DraftRevision+1, actor.ID, time.Now())
	if err != nil || published.DraftRevision != before.DraftRevision+1 || published.PublishedRevision != before.PublishedRevision+1 {
		t.Fatal("publication did not bind winning draft", published, err)
	}
}

func TestOnboardingDraftResponseDecodeFailureCannotCommitMutation(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	actor := createTestAdmin(t, s, 970004)
	// A valid JSON value can still have a shape that the typed bundle cannot read.
	if _, err := s.DB().ExecContext(ctx, `UPDATE onboarding_content SET published_json='[]' WHERE kind='welcome'`); err != nil {
		t.Fatal(err)
	}
	var before, after string
	if err := s.DB().QueryRowContext(ctx, `SELECT row_to_json(c)::text FROM onboarding_content c WHERE kind='welcome'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveOnboardingDraft(ctx, onboarding.KindWelcome, onboardingAdminTestContent("new draft"), 1, actor.ID, time.Now()); err == nil || errors.Is(err, ErrConflict) {
		t.Fatal("invalid retained bundle did not fail decoding", err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT row_to_json(c)::text FROM onboarding_content c WHERE kind='welcome'`).Scan(&after); err != nil || before != after {
		t.Fatal("failed response committed a new draft", err)
	}
}

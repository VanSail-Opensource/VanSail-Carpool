package database

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/onboarding"
)

const onboardingBundleColumns = `kind,draft_json,published_json,draft_revision,published_revision`

// SaveOnboardingDraft commits the revision, current administrator authorization
// and audit together, returning the bundle that this transaction actually wrote.
func (s *Store) retryAttempt_SaveOnboardingDraft(ctx context.Context, kind string, content onboarding.Content, expectedRevision int, actorID string, now time.Time) (onboarding.Bundle, error) {
	if err := onboarding.Validate(kind, content); err != nil {
		return onboarding.Bundle{}, err
	}
	encoded, err := json.Marshal(content)
	if err != nil {
		return onboarding.Bundle{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return onboarding.Bundle{}, err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return onboarding.Bundle{}, err
	}
	bundle, err := scanOnboardingBundle(tx.QueryRowContext(ctx, `UPDATE onboarding_content
		SET draft_json=$1,draft_revision=draft_revision+1,updated_at=$2,updated_by=$3
		WHERE kind=$4 AND draft_revision=$5 RETURNING `+onboardingBundleColumns,
		string(encoded), stamp(now), actorID, kind, expectedRevision))
	if errors.Is(err, ErrNotFound) {
		return onboarding.Bundle{}, ErrConflict
	}
	if err != nil {
		return onboarding.Bundle{}, err
	}
	detail := map[string]int{"draftRevision": bundle.DraftRevision, "publishedRevision": bundle.PublishedRevision}
	if err = catalogAdminAuditTx(ctx, tx, actorID, "onboarding.draft.update", "onboarding_content", kind, detail, now); err != nil {
		return onboarding.Bundle{}, err
	}
	if err = tx.Commit(); err != nil {
		return onboarding.Bundle{}, err
	}
	return bundle, nil
}

// PublishOnboarding serializes publication with draft writers. Republishing an
// unchanged draft retains its published revision and creates no duplicate audit.
func (s *Store) retryAttempt_PublishOnboarding(ctx context.Context, kind string, expectedDraftRevision int, actorID string, now time.Time) (onboarding.Bundle, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return onboarding.Bundle{}, err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return onboarding.Bundle{}, err
	}
	bundle, err := scanOnboardingBundle(tx.QueryRowContext(ctx, `SELECT `+onboardingBundleColumns+`
		FROM onboarding_content WHERE kind=$1 FOR UPDATE`, kind))
	if err != nil {
		return onboarding.Bundle{}, err
	}
	if bundle.DraftRevision != expectedDraftRevision {
		return onboarding.Bundle{}, ErrConflict
	}
	if err = onboarding.Validate(kind, bundle.Draft); err != nil {
		return onboarding.Bundle{}, err
	}
	draftJSON, err := json.Marshal(bundle.Draft)
	if err != nil {
		return onboarding.Bundle{}, err
	}
	publishedJSON, err := json.Marshal(bundle.Published)
	if err != nil {
		return onboarding.Bundle{}, err
	}
	if !slices.Equal(draftJSON, publishedJSON) {
		bundle, err = scanOnboardingBundle(tx.QueryRowContext(ctx, `UPDATE onboarding_content
			SET published_json=draft_json,published_revision=published_revision+1,published_at=$1,updated_by=$2
			WHERE kind=$3 AND draft_revision=$4 RETURNING `+onboardingBundleColumns,
			stamp(now), actorID, kind, expectedDraftRevision))
		if err != nil {
			return onboarding.Bundle{}, err
		}
		detail := map[string]int{"draftRevision": bundle.DraftRevision, "publishedRevision": bundle.PublishedRevision}
		if err = catalogAdminAuditTx(ctx, tx, actorID, "onboarding.publish", "onboarding_content", kind, detail, now); err != nil {
			return onboarding.Bundle{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return onboarding.Bundle{}, err
	}
	return bundle, nil
}

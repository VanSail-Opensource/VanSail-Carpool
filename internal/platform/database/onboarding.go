package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/onboarding"
)

func (s *Store) OnboardingBundle(ctx context.Context, kind string) (onboarding.Bundle, error) {
	return scanOnboardingBundle(s.db.QueryRowContext(ctx, `SELECT kind,draft_json,published_json,draft_revision,published_revision FROM onboarding_content WHERE kind=?`, kind))
}

func scanOnboardingBundle(row rowScanner) (onboarding.Bundle, error) {
	var bundle onboarding.Bundle
	var draftJSON, publishedJSON string
	if err := row.Scan(&bundle.Kind, &draftJSON, &publishedJSON, &bundle.DraftRevision, &bundle.PublishedRevision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return onboarding.Bundle{}, ErrNotFound
		}
		return onboarding.Bundle{}, err
	}
	if err := json.Unmarshal([]byte(draftJSON), &bundle.Draft); err != nil {
		return onboarding.Bundle{}, fmt.Errorf("decode onboarding draft: %w", err)
	}
	if err := json.Unmarshal([]byte(publishedJSON), &bundle.Published); err != nil {
		return onboarding.Bundle{}, fmt.Errorf("decode published onboarding content: %w", err)
	}
	return bundle, nil
}

func (s *Store) PublishedOnboarding(ctx context.Context, locale string) (onboarding.Published, error) {
	welcome, err := s.OnboardingBundle(ctx, onboarding.KindWelcome)
	if err != nil {
		return onboarding.Published{}, err
	}
	agreements, err := s.OnboardingBundle(ctx, onboarding.KindAgreements)
	if err != nil {
		return onboarding.Published{}, err
	}
	messages, err := onboarding.WelcomeForLocale(welcome.Published, locale)
	if err != nil {
		return onboarding.Published{}, err
	}
	agreementItems, err := onboarding.AgreementsForLocale(agreements.Published, locale)
	if err != nil {
		return onboarding.Published{}, err
	}
	return onboarding.Published{Locale: onboarding.SupportedLocale(locale), WelcomeRevision: welcome.PublishedRevision,
		AgreementRevision: agreements.PublishedRevision, Welcome: messages, Agreements: agreementItems}, nil
}

func (s *Store) SaveOnboardingDraft(ctx context.Context, kind string, content onboarding.Content, expectedRevision int, actorID string, now time.Time) (onboarding.Bundle, error) {
	if err := onboarding.Validate(kind, content); err != nil {
		return onboarding.Bundle{}, err
	}
	encoded, err := json.Marshal(content)
	if err != nil {
		return onboarding.Bundle{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE onboarding_content SET draft_json=?,draft_revision=draft_revision+1,updated_at=?,updated_by=? WHERE kind=? AND draft_revision=?`, string(encoded), stamp(now), actorID, kind, expectedRevision)
	if err != nil {
		return onboarding.Bundle{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return onboarding.Bundle{}, ErrConflict
	}
	return s.OnboardingBundle(ctx, kind)
}

func (s *Store) PublishOnboarding(ctx context.Context, kind string, expectedDraftRevision int, actorID string, now time.Time) (onboarding.Bundle, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return onboarding.Bundle{}, err
	}
	defer func() { _ = tx.Rollback() }()
	bundle, err := scanOnboardingBundle(tx.QueryRowContext(ctx, `SELECT kind,draft_json,published_json,draft_revision,published_revision FROM onboarding_content WHERE kind=?`, kind))
	if err != nil {
		return onboarding.Bundle{}, err
	}
	if bundle.DraftRevision != expectedDraftRevision {
		return onboarding.Bundle{}, ErrConflict
	}
	if err := onboarding.Validate(kind, bundle.Draft); err != nil {
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
	changed := !slices.Equal(draftJSON, publishedJSON)
	if changed {
		if _, err := tx.ExecContext(ctx, `UPDATE onboarding_content SET published_json=draft_json,published_revision=published_revision+1,published_at=?,updated_by=? WHERE kind=? AND draft_revision=?`, stamp(now), actorID, kind, expectedDraftRevision); err != nil {
			return onboarding.Bundle{}, err
		}
		if kind == onboarding.KindAgreements {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET onboarding_state='agreement',updated_at=? WHERE onboarding_state='complete'`, stamp(now)); err != nil {
				return onboarding.Bundle{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return onboarding.Bundle{}, err
	}
	return s.OnboardingBundle(ctx, kind)
}

func (s *Store) CurrentAgreementContract(ctx context.Context) (int, []string, error) {
	bundle, err := s.OnboardingBundle(ctx, onboarding.KindAgreements)
	if err != nil {
		return 0, nil, err
	}
	ids, err := onboarding.AgreementIDs(bundle.Published)
	return bundle.PublishedRevision, ids, err
}

func (s *Store) CompleteOnboardingRevision(ctx context.Context, userID string, revision int, agreementIDs []string, acceptedAt time.Time) (model.User, error) {
	provided := append([]string(nil), agreementIDs...)
	slices.Sort(provided)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.User{}, err
	}
	defer func() { _ = tx.Rollback() }()
	bundle, err := scanOnboardingBundle(tx.QueryRowContext(ctx, `SELECT kind,draft_json,published_json,draft_revision,published_revision FROM onboarding_content WHERE kind=?`, onboarding.KindAgreements))
	if err != nil {
		return model.User{}, err
	}
	requiredIDs, err := onboarding.AgreementIDs(bundle.Published)
	if err != nil {
		return model.User{}, err
	}
	slices.Sort(requiredIDs)
	if revision != bundle.PublishedRevision || !slices.Equal(provided, requiredIDs) {
		return model.User{}, ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET onboarding_state='complete',policy_accepted_at=?,accepted_agreement_revision=?,recovery_reason='',updated_at=? WHERE id=? AND onboarding_state='agreement'`, stamp(acceptedAt), revision, stamp(acceptedAt), userID)
	if err != nil {
		return model.User{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return model.User{}, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM panel_entry_verification WHERE telegram_id=(SELECT telegram_id FROM users WHERE id=?)`, userID); err != nil {
		return model.User{}, err
	}
	var purchased int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=? AND status IN ('active','activating','queued') AND valid_until>?)`, userID, stamp(acceptedAt)).Scan(&purchased); err != nil {
		return model.User{}, err
	}
	if purchased == 1 {
		if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+userID+`"}`, acceptedAt, acceptedAt); err != nil {
			return model.User{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.User{}, err
	}
	return s.UserByID(ctx, userID)
}

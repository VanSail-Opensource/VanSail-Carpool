package database

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/onboarding"
)

func onboardingAdminTestContent(text string) onboarding.Content {
	encoded, _ := json.Marshal([]onboarding.WelcomeMessage{{ID: "welcome", Text: text}})
	return onboarding.Content{"en": encoded, "zh-CN": append(json.RawMessage(nil), encoded...)}
}

func TestOnboardingWritesRequireCurrentAdministrator(t *testing.T) {
	for _, state := range []string{"demoted", "purging", "blocked", "empty", "missing"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			ctx, s := context.Background(), newTestStore(t)
			actor := createTestAdmin(t, s, 970001)
			before, err := s.OnboardingBundle(ctx, onboarding.KindWelcome)
			if err != nil {
				t.Fatal(err)
			}
			id := actor.ID
			switch state {
			case "demoted":
				_, err = s.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, id)
			case "purging":
				_, err = s.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, id)
			case "blocked":
				_, err = s.DB().ExecContext(ctx, `UPDATE users SET bot_blocked=1 WHERE id=$1`, id)
			case "empty":
				id = ""
			case "missing":
				id = "unknown-administrator"
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.SaveOnboardingDraft(ctx, before.Kind, onboardingAdminTestContent("private draft"), before.DraftRevision, id, time.Now()); !errors.Is(err, ErrConflict) {
				t.Fatal("draft accepted stale authority", err)
			}
			if _, err = s.PublishOnboarding(ctx, before.Kind, before.DraftRevision, id, time.Now()); !errors.Is(err, ErrConflict) {
				t.Fatal("publication accepted stale authority", err)
			}
			after, err := s.OnboardingBundle(ctx, before.Kind)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("refused authority changed content", after, err)
			}
			var count int
			if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM audit_events WHERE target_type='onboarding_content'`).Scan(&count); err != nil || count != 0 {
				t.Fatal("refused authority left audit", count, err)
			}
		})
	}
}

func TestOnboardingAuditFailureRollsBackDraftAndPublication(t *testing.T) {
	for _, publish := range []bool{false, true} {
		t.Run(map[bool]string{false: "draft", true: "publish"}[publish], func(t *testing.T) {
			t.Parallel()
			ctx, s := context.Background(), newTestStore(t)
			actor := createTestAdmin(t, s, 970002)
			before, err := s.OnboardingBundle(ctx, onboarding.KindWelcome)
			if err != nil {
				t.Fatal(err)
			}
			const privateContent = "unpublished-private-content"
			if publish {
				before, err = s.SaveOnboardingDraft(ctx, before.Kind, onboardingAdminTestContent(privateContent), before.DraftRevision, actor.ID, time.Now())
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, statement := range []string{
				`CREATE FUNCTION reject_onboarding_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'onboarding audit unavailable'; END $$`,
				`CREATE TRIGGER reject_onboarding_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.target_type='onboarding_content') EXECUTE FUNCTION reject_onboarding_audit()`,
			} {
				if _, err = s.DB().ExecContext(ctx, statement); err != nil {
					t.Fatal(err)
				}
			}
			write := func() (onboarding.Bundle, error) {
				if publish {
					return s.PublishOnboarding(ctx, before.Kind, before.DraftRevision, actor.ID, time.Now())
				}
				return s.SaveOnboardingDraft(ctx, before.Kind, onboardingAdminTestContent(privateContent), before.DraftRevision, actor.ID, time.Now())
			}
			if _, err = write(); err == nil || !strings.Contains(err.Error(), "onboarding audit unavailable") {
				t.Fatal("mutation committed without audit", err)
			}
			after, err := s.OnboardingBundle(ctx, before.Kind)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("audit failure changed bundle", after, err)
			}
			if _, err = s.DB().ExecContext(ctx, `DROP TRIGGER reject_onboarding_audit ON audit_events`); err != nil {
				t.Fatal(err)
			}
			written, err := write()
			if err != nil {
				t.Fatal(err)
			}
			stored, err := s.OnboardingBundle(ctx, before.Kind)
			if err != nil || !reflect.DeepEqual(written, stored) {
				t.Fatal("write returned another bundle", written, stored, err)
			}
			action := "onboarding.draft.update"
			if publish {
				action = "onboarding.publish"
				replayed, err := s.PublishOnboarding(ctx, written.Kind, written.DraftRevision, actor.ID, time.Now())
				if err != nil || !reflect.DeepEqual(replayed, written) {
					t.Fatal("unchanged publication advanced revision", replayed, err)
				}
			}
			var count int
			var detail string
			if err = s.DB().QueryRowContext(ctx, `SELECT count(*),COALESCE(min(detail),'') FROM audit_events
				WHERE actor_user_id=$1 AND action=$2 AND target_type='onboarding_content' AND target_id=$3`, actor.ID, action, before.Kind).Scan(&count, &detail); err != nil || count != 1 || strings.Contains(detail, privateContent) {
				t.Fatal("missing, repeated or unsanitized audit", count, detail, err)
			}
		})
	}
}
